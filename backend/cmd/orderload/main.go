package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"statebroker/sbis/internal/adapters/kfb"
	"statebroker/sbis/internal/loadtest"
)

type config struct {
	participants                                                  []loadtest.Participant
	participantError                                              error
	schedule                                                      []int
	runID                                                         string
	waitingRoom                                                   bool
	baseURL, token, account, client, symbol, board, price, outDir string
	rate, workers, verifyWorkers, verifyRetries                   int
	duration, timeout, settleWait                                 time.Duration
	execute, allowLive                                            bool
}

type result struct {
	Quantity        string        `json:"quantity"`
	AccountID       string        `json:"account_id"`
	ClientCode      string        `json:"client_code"`
	Sequence        int           `json:"sequence"`
	ClientID        string        `json:"client_order_id"`
	ServerID        string        `json:"server_order_id,omitempty"`
	Side            string        `json:"side"`
	HTTPCode        int           `json:"http_code"`
	Latency         time.Duration `json:"-"`
	Error           string        `json:"error,omitempty"`
	Verified        bool          `json:"verified"`
	ExchangeReached bool          `json:"exchange_reached"`
	Status          string        `json:"status,omitempty"`
	Price           string        `json:"price"`
	RejectReason    string        `json:"reject_reason,omitempty"`
}

type createResponse struct {
	Order struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		RejectReason string `json:"reject_reason"`
	} `json:"order"`
}

func main() {
	cfg := parseConfig()
	if err := validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "ОШИБКА:", err)
		os.Exit(2)
	}
	total := cfg.rate * int(cfg.duration/time.Second)
	cfg.schedule = loadtest.ClientSchedule(total, len(cfg.participants))
	if !cfg.execute {
		fmt.Printf("DRY-RUN: %d заявок (%d/с, %s) -> %s\n", total, cfg.rate, cfg.duration, cfg.baseURL)
		fmt.Println("Для реальной отправки добавьте --execute или LOAD_EXECUTE=true.")
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client := &http.Client{Timeout: cfg.timeout, Transport: &http.Transport{MaxIdleConns: cfg.workers * 2, MaxIdleConnsPerHost: cfg.workers, MaxConnsPerHost: cfg.workers}}
	if os.Getenv("LOAD_KFB_LIMIT") == "true" {
		options, err := redis.ParseURL(os.Getenv("REDIS_URL"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "KFB limiter requires REDIS_URL:", err)
			os.Exit(2)
		}
		rdb := redis.NewClient(options)
		defer rdb.Close()
		gateClient := kfb.New(cfg.baseURL, cfg.token)
		tradingRPS, _ := strconv.Atoi(os.Getenv("KFB_TRADING_RPS"))
		readRPS, _ := strconv.Atoi(os.Getenv("KFB_READ_RPS"))
		gateClient.ConfigureRateLimit(rdb, tradingRPS, readRPS)
		client.Transport = gateClient.LimitedTransport(client.Transport)
	}
	jobs := make(chan int, cfg.workers*2)
	results := make(chan result, cfg.workers*2)
	var wg sync.WaitGroup
	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seq := range jobs {
				results <- submit(ctx, client, cfg, seq)
			}
		}()
	}
	go func() {
		defer close(jobs)
		interval := time.Second / time.Duration(cfg.rate)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for i := 0; i < total; i++ {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
			if i+1 < total {
				select {
				case <-ticker.C:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	started := time.Now()
	all := make([]result, 0, total)
	var done atomic.Int64
	progress := time.NewTicker(time.Second)
	defer progress.Stop()
	collecting := true
	for collecting {
		select {
		case r, ok := <-results:
			if !ok {
				collecting = false
				break
			}
			all = append(all, r)
			emitResult("LOAD_RESULT", r)
			done.Add(1)
		case <-progress.C:
			fmt.Printf("отправлено %d/%d\r", done.Load(), total)
		case <-ctx.Done():
			collecting = false
		}
	}
	fmt.Printf("отправлено %d/%d за %s\n", len(all), total, time.Since(started).Round(time.Millisecond))
	if cfg.settleWait > 0 {
		fmt.Printf("ожидание обработки %s...\n", cfg.settleWait)
		select {
		case <-time.After(cfg.settleWait):
		case <-ctx.Done():
		}
	}
	verify(ctx, client, cfg, all)
	for _, r := range all {
		emitResult("LOAD_RESULT_UPDATE", r)
	}
	if err := writeReports(cfg, all, started); err != nil {
		fmt.Fprintln(os.Stderr, "отчёт:", err)
	}
	if !printSummary(all, cfg.waitingRoom) {
		os.Exit(1)
	}
}

func emitResult(prefix string, r result) {
	payload := map[string]any{}
	encoded, _ := json.Marshal(r)
	_ = json.Unmarshal(encoded, &payload)
	payload["latency_ms"] = float64(r.Latency.Microseconds()) / 1000
	raw, _ := json.Marshal(payload)
	fmt.Printf("%s %s\n", prefix, raw)
}

func parseConfig() config {
	c := config{}
	if raw := os.Getenv("LOAD_PARTICIPANTS"); raw != "" {
		c.participantError = json.Unmarshal([]byte(raw), &c.participants)
	}
	c.runID = env("LOAD_RUN_ID", "")
	c.waitingRoom = envBool("LOAD_WAITING_ROOM", false)
	flag.StringVar(&c.baseURL, "base-url", env("LOAD_BASE_URL", "http://localhost:18080"), "URL нашего API или тестового шлюза биржи")
	flag.StringVar(&c.token, "token", env("LOAD_TOKEN", ""), "тестовый sk_test_ токен")
	flag.StringVar(&c.account, "account", env("LOAD_ACCOUNT", "1-3101-006"), "торговый счёт")
	flag.StringVar(&c.client, "client", env("LOAD_CLIENT", "006"), "код клиента")
	flag.StringVar(&c.symbol, "symbol", env("LOAD_SYMBOL", "KG0105166315"), "инструмент")
	flag.StringVar(&c.board, "board", env("LOAD_BOARD", "LISTEDSTOCKK"), "режим торгов")
	flag.StringVar(&c.price, "price", env("LOAD_PRICE", "400"), "лимитная цена")
	flag.StringVar(&c.outDir, "out", env("LOAD_OUT", "/reports"), "каталог отчётов")
	flag.IntVar(&c.rate, "rate", envInt("LOAD_RATE", 100), "заявок в секунду")
	flag.IntVar(&c.workers, "workers", envInt("LOAD_WORKERS", 100), "параллельные отправители")
	flag.IntVar(&c.verifyWorkers, "verify-workers", envInt("LOAD_VERIFY_WORKERS", 50), "параллельные проверки")
	flag.IntVar(&c.verifyRetries, "verify-retries", envInt("LOAD_VERIFY_RETRIES", 5), "повторы проверки")
	flag.DurationVar(&c.duration, "duration", envDuration("LOAD_DURATION", 10*time.Second), "длительность")
	flag.DurationVar(&c.timeout, "timeout", envDuration("LOAD_TIMEOUT", 10*time.Second), "HTTP timeout")
	flag.DurationVar(&c.settleWait, "settle-wait", envDuration("LOAD_SETTLE_WAIT", 5*time.Second), "пауза перед сверкой")
	flag.BoolVar(&c.execute, "execute", envBool("LOAD_EXECUTE", false), "реально отправлять заявки")
	flag.BoolVar(&c.allowLive, "allow-live-token", envBool("LOAD_ALLOW_LIVE", false), "разрешить sk_live_ токен")
	flag.Parse()
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	return c
}

func validate(c config) error {
	if c.participantError != nil {
		return fmt.Errorf("LOAD_PARTICIPANTS: %w", c.participantError)
	}
	for _, participant := range c.participants {
		if participant.AccountID == "" || participant.ClientCode == "" {
			return errors.New("каждому тестовому клиенту необходимы account_id и client_code")
		}
	}
	if c.rate < 1 || c.rate > 100000 {
		return errors.New("rate должен быть 1..100000")
	}
	if c.duration < time.Second || c.workers < 1 {
		return errors.New("неверные duration/workers")
	}
	if c.execute && c.token == "" {
		return errors.New("LOAD_TOKEN обязателен")
	}
	if c.execute && strings.HasPrefix(c.token, "sk_live_") && !c.allowLive {
		return errors.New("live-токен заблокирован; используйте только тестовый токен или явно --allow-live-token")
	}
	if c.execute && !strings.HasPrefix(c.token, "sk_test_") && !c.allowLive {
		return errors.New("разрешены только sk_test_ токены")
	}
	if price, err := decimal.NewFromString(c.price); err != nil || !price.IsPositive() {
		return errors.New("price должна быть положительным числом")
	}
	return nil
}

func submit(ctx context.Context, client *http.Client, c config, seq int) result {
	side := "BUY"
	if seq%2 == 1 {
		side = "SELL"
	}
	clientID := fmt.Sprintf("LOAD-%d-%08d", time.Now().UnixNano(), seq)
	if c.runID != "" {
		clientID = fmt.Sprintf("LOAD-%s-%08d", c.runID, seq)
	}
	price := variedPrice(c.price, seq)
	quantity := strconv.Itoa(100 + rand.IntN(901))
	account, code := c.account, c.client
	if len(c.participants) > 0 {
		participant := c.participants[c.schedule[seq]]
		account, code = participant.AccountID, participant.ClientCode
	}
	body := map[string]any{"account_id": account, "client_code": code, "client_order_id": clientID, "symbol": c.symbol, "board": c.board, "side": side, "order_type": "LIMIT", "time_in_force": "DAY", "quantity": quantity, "price": price}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/orders", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", clientID)
	start := time.Now()
	response, err := client.Do(req)
	r := result{Sequence: seq, ClientID: clientID, AccountID: account, ClientCode: code, Side: side, Price: price, Quantity: quantity, Latency: time.Since(start)}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	defer response.Body.Close()
	r.HTTPCode = response.StatusCode
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != 200 && response.StatusCode != 201 {
		r.Error = strings.TrimSpace(string(payload))
		return r
	}
	var decoded createResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		r.Error = "неверный JSON подтверждения: " + err.Error()
		return r
	}
	r.ServerID, r.Status = decoded.Order.ID, decoded.Order.Status
	if r.ServerID == "" {
		r.Error = "подтверждение без order.id"
	}
	return r
}

// variedPrice равномерно перебирает целые проценты от -5% до +5%.
// Формула зависит только от номера заявки, поэтому нагрузочный прогон воспроизводим.
func variedPrice(base string, sequence int) string {
	value, err := decimal.NewFromString(base)
	if err != nil {
		return base
	}
	percent := int64((sequence*7)%11 - 5)
	factor := decimal.NewFromInt(100 + percent).Div(decimal.NewFromInt(100))
	return value.Mul(factor).Round(2).String()
}

func verify(ctx context.Context, client *http.Client, c config, all []result) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < c.verifyWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if all[idx].ServerID == "" {
					continue
				}
				for attempt := 0; attempt < c.verifyRetries; attempt++ {
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/orders/"+all[idx].ServerID, nil)
					req.Header.Set("Authorization", "Bearer "+c.token)
					res, err := client.Do(req)
					if err == nil {
						if res.StatusCode == http.StatusOK {
							all[idx].Verified = true
							var detail createResponse
							if json.NewDecoder(res.Body).Decode(&detail) == nil && detail.Order.Status != "" {
								all[idx].Status = detail.Order.Status
								all[idx].RejectReason = detail.Order.RejectReason
							}
							res.Body.Close()
							if c.waitingRoom || !isInternalQueueStatus(all[idx].Status) {
								all[idx].ExchangeReached = isExchangeConfirmedStatus(all[idx].Status)
								break
							}
						} else {
							_, _ = io.Copy(io.Discard, res.Body)
							res.Body.Close()
						}
					}
					time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				}
			}
		}()
	}
	for i := range all {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func writeReports(c config, all []result, started time.Time) error {
	if err := os.MkdirAll(c.outDir, 0755); err != nil {
		return err
	}
	stamp := started.UTC().Format("20060102T150405Z")
	raw, _ := json.MarshalIndent(all, "", "  ")
	if err := os.WriteFile(filepath.Join(c.outDir, "orders-"+stamp+".json"), raw, 0644); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(c.outDir, "orders-"+stamp+".csv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"sequence", "client_order_id", "server_order_id", "account_id", "client_code", "side", "quantity", "price", "http_code", "latency_ms", "verified", "exchange_reached", "status", "error"})
	for _, r := range all {
		_ = w.Write([]string{strconv.Itoa(r.Sequence), r.ClientID, r.ServerID, r.AccountID, r.ClientCode, r.Side, r.Quantity, r.Price, strconv.Itoa(r.HTTPCode), strconv.FormatInt(r.Latency.Milliseconds(), 10), strconv.FormatBool(r.Verified), strconv.FormatBool(r.ExchangeReached), r.Status, r.Error})
	}
	return nil
}

func printSummary(all []result, waitingRoom ...bool) bool {
	var accepted, rejected, transport, verified, lost, exchangeReached, pending, adapterFailed int
	ids := map[string]int{}
	httpCodes := map[int]int{}
	reasons := map[string]int{}
	lat := make([]int64, 0, len(all))
	for _, r := range all {
		if r.HTTPCode > 0 {
			httpCodes[r.HTTPCode]++
			lat = append(lat, r.Latency.Microseconds())
		}
		if r.HTTPCode == 200 || r.HTTPCode == 201 {
			accepted++
			ids[r.ServerID]++
			if r.Verified {
				verified++
			} else {
				lost++
			}
			if r.ExchangeReached {
				exchangeReached++
			}
			if isInternalQueueStatus(r.Status) {
				pending++
			}
			if strings.EqualFold(r.Status, "FAILED") {
				adapterFailed++
			}
		} else if r.HTTPCode > 0 {
			rejected++
			reasons[rejectionReason(r.Error)]++
		} else {
			transport++
		}
	}
	dupes := 0
	for id, n := range ids {
		if id != "" && n > 1 {
			dupes += n - 1
		}
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	pct := func(p float64) float64 {
		if len(lat) == 0 {
			return 0
		}
		return float64(lat[min(len(lat)-1, int(float64(len(lat)-1)*p))]) / 1000
	}
	fmt.Printf("\nИТОГ: отправлено=%d принято=%d отклонено_HTTP=%d транспортных_ошибок=%d найдено_в_OMS=%d подтверждено_биржей=%d в_очереди=%d ошибки_адаптера=%d потеряно=%d дубли=%d\n", len(all), accepted, rejected, transport, verified, exchangeReached, pending, adapterFailed, lost, dupes)
	if rejected > 0 {
		codes := make([]int, 0, len(httpCodes))
		for code := range httpCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		parts := make([]string, 0, len(codes))
		for _, code := range codes {
			parts = append(parts, fmt.Sprintf("HTTP %d — %d", code, httpCodes[code]))
		}
		fmt.Println("Ответы:", strings.Join(parts, ", "))
		type reasonCount struct {
			reason string
			count  int
		}
		ranked := make([]reasonCount, 0, len(reasons))
		for reason, count := range reasons {
			ranked = append(ranked, reasonCount{reason, count})
		}
		sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })
		for i := 0; i < min(3, len(ranked)); i++ {
			fmt.Printf("Причина отказа (%d): %s\n", ranked[i].count, ranked[i].reason)
		}
	}
	fmt.Printf("Задержка POST: p50=%.2fms p95=%.2fms p99=%.2fms\n", pct(.50), pct(.95), pct(.99))
	queueOK := len(waitingRoom) > 0 && waitingRoom[0]
	return rejected == 0 && lost == 0 && dupes == 0 && transport == 0 && adapterFailed == 0 && ((queueOK && exchangeReached+pending == accepted) || (pending == 0 && exchangeReached == accepted))
}

func rejectionReason(raw string) string {
	var body struct {
		Error   any `json:"error"`
		Details struct {
			Code string `json:"code"`
		} `json:"details"`
	}
	if json.Unmarshal([]byte(raw), &body) == nil {
		message := fmt.Sprint(body.Error)
		if message != "" && message != "<nil>" {
			if body.Details.Code != "" {
				return body.Details.Code + ": " + message
			}
			return message
		}
	}
	if len(raw) > 240 {
		return raw[:240] + "…"
	}
	if raw == "" {
		return "Пустой ответ"
	}
	return raw
}

func isInternalQueueStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "", "NEW", "VALIDATING", "VALIDATED", "PENDING", "QUEUED", "ROUTING", "SENT":
		return true
	}
	return false
}

func isExchangeConfirmedStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "ACKNOWLEDGED", "PARTIALLY_FILLED", "FILLED", "CANCEL_PENDING", "CANCELLED", "REJECTED", "EXPIRED":
		return true
	}
	return false
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	v, e := strconv.Atoi(env(k, ""))
	if e == nil {
		return v
	}
	return d
}
func envBool(k string, d bool) bool {
	v, e := strconv.ParseBool(env(k, ""))
	if e == nil {
		return v
	}
	return d
}
func envDuration(k string, d time.Duration) time.Duration {
	v, e := time.ParseDuration(env(k, ""))
	if e == nil {
		return v
	}
	return d
}
