package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"statebroker/sbis/internal/loadtest"
	"statebroker/sbis/internal/platform/hash"
	"statebroker/sbis/internal/platform/httpx"
)

type loadTestRun struct {
	Participants         []loadtest.Participant `json:"participants,omitempty"`
	BrokerID             uuid.UUID              `json:"broker_id"`
	TokenID              uuid.UUID              `json:"token_id"`
	ID                   string                 `json:"id"`
	Target               string                 `json:"target"`
	Status               string                 `json:"status"`
	Rate                 int                    `json:"rate"`
	DurationSeconds      int                    `json:"duration_seconds"`
	Workers              int                    `json:"workers"`
	Planned              int                    `json:"planned"`
	Sent                 int                    `json:"sent"`
	StartedAt            time.Time              `json:"started_at"`
	FinishedAt           *time.Time             `json:"finished_at,omitempty"`
	Output               string                 `json:"output"`
	Error                string                 `json:"error,omitempty"`
	Account              string                 `json:"account"`
	ClientCode           string                 `json:"client_code"`
	Symbol               string                 `json:"symbol"`
	Board                string                 `json:"board"`
	Queue                *loadQueueStats        `json:"queue,omitempty"`
	GenerationFinishedAt *time.Time             `json:"generation_finished_at,omitempty"`
	cancel               context.CancelFunc
	requests             map[int]loadTestRequestResult
}

type loadTestRequestResult struct {
	VenueOrderID      string     `json:"venue_order_id,omitempty"`
	ExchangeOrderID   string     `json:"exchange_order_id,omitempty"`
	VenueStatus       string     `json:"venue_status,omitempty"`
	VenueCheckedAt    *time.Time `json:"venue_checked_at,omitempty"`
	VenueCheckError   string     `json:"venue_check_error,omitempty"`
	RemainingQuantity string     `json:"remaining_quantity,omitempty"`
	Quantity          string     `json:"quantity,omitempty"`
	AccountID         string     `json:"account_id,omitempty"`
	ClientCode        string     `json:"client_code,omitempty"`
	Sequence          int        `json:"sequence"`
	ClientOrderID     string     `json:"client_order_id"`
	ServerOrderID     string     `json:"server_order_id,omitempty"`
	Side              string     `json:"side"`
	Price             string     `json:"price"`
	HTTPCode          int        `json:"http_code"`
	LatencyMS         float64    `json:"latency_ms"`
	Error             string     `json:"error,omitempty"`
	Verified          bool       `json:"verified"`
	ExchangeReached   bool       `json:"exchange_reached"`
	Status            string     `json:"status,omitempty"`
	RejectReason      string     `json:"reject_reason,omitempty"`
}

type loadTestRequest struct {
	Target          string `json:"target"`
	Rate            int    `json:"rate"`
	DurationSeconds int    `json:"duration_seconds"`
	Workers         int    `json:"workers"`
	VerifyWorkers   int    `json:"verify_workers"`
	Account         string `json:"account"`
	ClientCode      string `json:"client_code"`
	Symbol          string `json:"symbol"`
	Board           string `json:"board"`
	Price           string `json:"price"`
	AdjustRateLimit bool   `json:"adjust_rate_limit"`
	Confirm         bool   `json:"confirm"`
}

var loadProgressRE = regexp.MustCompile(`отправлено\s+(\d+)/(\d+)`)

func (a *App) handleCurrentLoadTest(w http.ResponseWriter, r *http.Request) {
	a.LoadMu.Lock()
	defer a.LoadMu.Unlock()
	if err := a.restoreLoadRun(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if a.LoadRun == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "IDLE"})
		return
	}
	if err := a.refreshLoadQueue(r.Context(), a.LoadRun); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.LoadRun)
}

func (a *App) handleLoadTestRequests(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	a.LoadMu.Lock()
	defer a.LoadMu.Unlock()
	if err := a.restoreLoadRun(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if a.LoadRun == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": []any{}, "total": 0, "page": page, "page_size": pageSize})
		return
	}
	if err := a.refreshLoadResults(r.Context(), a.LoadRun); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	errorsOnly := r.URL.Query().Get("kind") == "errors"
	keys := make([]int, 0, len(a.LoadRun.requests))
	for sequence, item := range a.LoadRun.requests {
		if errorsOnly && !isLoadRequestError(item) {
			continue
		}
		keys = append(keys, sequence)
	}
	sort.Ints(keys)
	total := len(keys)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := min(start+pageSize, total)
	items := make([]loadTestRequestResult, 0, end-start)
	for _, sequence := range keys[start:end] {
		items = append(items, a.LoadRun.requests[sequence])
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func isLoadRequestError(item loadTestRequestResult) bool {
	status := strings.ToUpper(item.Status)
	return item.Error != "" || item.HTTPCode >= 400 || status == "FAILED" || status == "REJECTED" || (item.RejectReason != "" && status != "QUEUED")
}

func (a *App) handleStartLoadTest(w http.ResponseWriter, r *http.Request) {
	var req loadTestRequest
	if err := decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Target == "" {
		req.Target = "broker"
	}
	if req.Rate < 1 || req.Rate > 100000 || req.DurationSeconds < 1 || req.DurationSeconds > 300 || req.Workers < 1 || req.Workers > 5000 {
		httpx.WriteError(w, r, httpx.E(http.StatusUnprocessableEntity, "INVALID_LOAD_PARAMETERS", "Проверьте параметры: скорость 1–100000, длительность 1–300 секунд, воркеры 1–5000"))
		return
	}
	if !req.Confirm {
		httpx.WriteError(w, r, httpx.E(http.StatusUnprocessableEntity, "CONFIRMATION_REQUIRED", "Подтвердите отправку реальных тестовых заявок"))
		return
	}
	if req.VerifyWorkers < 1 {
		req.VerifyWorkers = min(req.Workers, 100)
	}

	baseURL, token := "http://127.0.0.1:8080", a.Cfg.GatewayDevToken
	defaults := []string{"1-3101-2040", "Kalys", "GBA02280810", "LIST_GBA", "100"}
	if req.Target == "exchange" {
		defaults = []string{"1-3101-2040", "Kalys", "GBA02280810", "LIST_GBA", "90"}
	} else if req.Target != "broker" {
		httpx.WriteError(w, r, httpx.E(http.StatusUnprocessableEntity, "INVALID_TARGET", "Выберите нашу систему или тестовый шлюз биржи"))
		return
	}
	if !strings.HasPrefix(token, "sk_test_") {
		httpx.WriteError(w, r, httpx.E(http.StatusUnprocessableEntity, "TEST_TOKEN_REQUIRED", "Нагрузочный тест разрешён только с тестовым токеном sk_test_"))
		return
	}
	values := []*string{&req.Account, &req.ClientCode, &req.Symbol, &req.Board, &req.Price}
	for i := range values {
		if *values[i] == "" {
			*values[i] = defaults[i]
		}
	}
	preflightClient := &http.Client{Timeout: 30 * time.Second}
	if err := preflightLoadTarget(r.Context(), baseURL, token, req, preflightClient); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if a.KFB == nil || !a.KFB.Configured() {
		httpx.WriteError(w, r, httpx.E(422, "LOAD_CLIENTS_UNAVAILABLE", "Шлюз КФБ не настроен для проверки клиентов"))
		return
	}
	positions, err := a.kfbPositions(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	participants, err := loadParticipants(positions.Positions, req.Account)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var tokenID, brokerID uuid.UUID
	if err := a.DB.QueryRow(r.Context(), `SELECT id,broker_id FROM api_tokens WHERE token_hash=$1`, hash.SHA256Hex(token)).Scan(&tokenID, &brokerID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	a.LoadMu.Lock()
	if err := a.restoreLoadRun(r.Context()); err != nil {
		a.LoadMu.Unlock()
		httpx.WriteError(w, r, err)
		return
	}
	if a.LoadRun != nil {
		if err := a.refreshLoadQueue(r.Context(), a.LoadRun); err != nil {
			a.LoadMu.Unlock()
			httpx.WriteError(w, r, err)
			return
		}
	}
	if a.LoadRun != nil && (a.LoadRun.Status == "RUNNING" || a.LoadRun.Status == "DRAINING" || (a.LoadRun.Queue != nil && a.LoadRun.Queue.Waiting > 0)) {
		a.LoadMu.Unlock()
		httpx.WriteError(w, r, httpx.E(http.StatusConflict, "LOAD_TEST_RUNNING", "Нагрузочный тест уже выполняется"))
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &loadTestRun{ID: uuid.NewString(), Target: req.Target, Status: "RUNNING", Rate: req.Rate, DurationSeconds: req.DurationSeconds, Workers: req.Workers, Planned: req.Rate * req.DurationSeconds, StartedAt: time.Now().UTC(), Account: req.Account, ClientCode: req.ClientCode, Symbol: req.Symbol, Board: req.Board, cancel: cancel, requests: make(map[int]loadTestRequestResult)}
	run.Participants, run.TokenID, run.BrokerID = participants, tokenID, brokerID
	if err := a.saveLoadRun(r.Context(), run); err != nil {
		cancel()
		a.LoadMu.Unlock()
		httpx.WriteError(w, r, err)
		return
	}
	a.LoadRun = run
	a.LoadMu.Unlock()

	if req.AdjustRateLimit {
		_, _ = a.DB.Exec(r.Context(), `UPDATE api_tokens SET rate_limit_rps=GREATEST(rate_limit_rps,$1), order_rate_limit_rps=GREATEST(order_rate_limit_rps,$1) WHERE token_hash=$2`, req.Rate+10, hash.SHA256Hex(token))
	}

	env := append(os.Environ(),
		"LOAD_BASE_URL="+strings.TrimRight(baseURL, "/"), "LOAD_TOKEN="+token,
		"LOAD_RATE="+strconv.Itoa(req.Rate), "LOAD_DURATION="+strconv.Itoa(req.DurationSeconds)+"s",
		"LOAD_WORKERS="+strconv.Itoa(req.Workers), "LOAD_VERIFY_WORKERS="+strconv.Itoa(req.VerifyWorkers),
		"LOAD_ACCOUNT="+req.Account, "LOAD_CLIENT="+req.ClientCode, "LOAD_SYMBOL="+req.Symbol,
		"LOAD_BOARD="+req.Board, "LOAD_PRICE="+req.Price, "LOAD_EXECUTE=true", "LOAD_OUT=/reports",
		"LOAD_RUN_ID="+run.ID, "LOAD_WAITING_ROOM=true",
	)
	participantJSON, _ := json.Marshal(participants)
	env = append(env, "LOAD_PARTICIPANTS="+string(participantJSON))
	cmd := exec.CommandContext(ctx, "/app/orderload")
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.failLoadRun(run.ID, err)
		httpx.WriteError(w, r, err)
		return
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		a.failLoadRun(run.ID, err)
		httpx.WriteError(w, r, err)
		return
	}
	go a.collectLoadRun(ctx, run.ID, cmd, stdout)
	a.Audit.Write(r.Context(), "LOAD_TEST_STARTED", "load_test", nil, nil, map[string]any{"id": run.ID, "target": req.Target, "rate": req.Rate, "duration_seconds": req.DurationSeconds})
	httpx.WriteJSON(w, http.StatusAccepted, run)
}

func preflightLoadTarget(ctx context.Context, baseURL, token string, req loadTestRequest, client *http.Client) error {
	get := func(path string) (map[string]any, error) {
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
		httpReq.Header.Set("Authorization", "Bearer "+token)
		res, err := client.Do(httpReq)
		if err != nil {
			return nil, httpx.E(http.StatusBadGateway, "LOAD_TARGET_UNAVAILABLE", "Не удалось подключиться к тестируемой системе: "+err.Error())
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if res.StatusCode != http.StatusOK {
			return nil, httpx.E(http.StatusBadGateway, "LOAD_PREFLIGHT_FAILED", fmt.Sprintf("Предварительная проверка вернула HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body))))
		}
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil {
			return nil, httpx.E(http.StatusBadGateway, "LOAD_PREFLIGHT_FAILED", "Тестируемая система вернула некорректный ответ")
		}
		return payload, nil
	}
	accounts, err := get("/api/v1/accounts")
	if err != nil {
		return err
	}
	accountOK := false
	available := make([]string, 0, 5)
	if rows, ok := accounts["accounts"].([]any); ok {
		for _, item := range rows {
			row, _ := item.(map[string]any)
			account := fmt.Sprint(row["account_id"])
			if account == "<nil>" || account == "" {
				account = fmt.Sprint(row["trading_account"])
			}
			clientCode := fmt.Sprint(row["client_code"])
			if account != "" && account != "<nil>" && len(available) < 5 {
				available = append(available, account)
			}
			if account == req.Account && clientCode == req.ClientCode {
				accountOK = true
			}
		}
	}
	if !accountOK {
		hint := "Доступные счета не найдены"
		if len(available) > 0 {
			hint = "Доступные счета: " + strings.Join(available, ", ")
		}
		return httpx.E(http.StatusUnprocessableEntity, "LOAD_ACCOUNT_NOT_FOUND", fmt.Sprintf("Счёт %s с кодом клиента %s недоступен выбранному тестовому токену. %s", req.Account, req.ClientCode, hint))
	}
	instruments, err := get("/api/v1/instruments")
	if err != nil {
		return err
	}
	instrumentOK := false
	if rows, ok := instruments["instruments"].([]any); ok {
		for _, item := range rows {
			row, _ := item.(map[string]any)
			symbols := []string{fmt.Sprint(row["symbol"]), fmt.Sprint(row["quik_symbol"]), fmt.Sprint(row["internal_symbol"]), fmt.Sprint(row["ticker"]), fmt.Sprint(row["isin"])}
			board := fmt.Sprint(row["board"])
			for _, symbol := range symbols {
				if symbol == req.Symbol && (board == req.Board || board == "" || board == "<nil>") {
					instrumentOK = true
				}
			}
		}
	}
	if !instrumentOK {
		return httpx.E(http.StatusUnprocessableEntity, "LOAD_INSTRUMENT_NOT_FOUND", fmt.Sprintf("Инструмент %s в режиме %s недоступен выбранному тестовому токену", req.Symbol, req.Board))
	}
	return nil
}

func (a *App) collectLoadRun(ctx context.Context, id string, cmd *exec.Cmd, output interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	scanner.Split(scanLinesAndReturns)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "LOAD_RESULT ") || strings.HasPrefix(line, "LOAD_RESULT_UPDATE ") {
			payload := strings.TrimPrefix(strings.TrimPrefix(line, "LOAD_RESULT_UPDATE "), "LOAD_RESULT ")
			var item loadTestRequestResult
			if json.Unmarshal([]byte(payload), &item) == nil {
				persistCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, persistErr := a.DB.Exec(persistCtx, `INSERT INTO load_test_results(run_id,sequence,payload) VALUES($1,$2,$3) ON CONFLICT(run_id,sequence) DO UPDATE SET payload=EXCLUDED.payload`, id, item.Sequence, []byte(payload))
				cancel()
				a.LoadMu.Lock()
				if a.LoadRun != nil && a.LoadRun.ID == id {
					a.LoadRun.requests[item.Sequence] = item
					a.LoadRun.Sent = max(a.LoadRun.Sent, len(a.LoadRun.requests))
					if persistErr != nil {
						a.LoadRun.Error = "Не удалось сохранить журнал запроса: " + persistErr.Error()
					}
				}
				a.LoadMu.Unlock()
			}
			continue
		}
		a.LoadMu.Lock()
		if a.LoadRun != nil && a.LoadRun.ID == id {
			a.LoadRun.Output += line + "\n"
			if len(a.LoadRun.Output) > 50000 {
				a.LoadRun.Output = a.LoadRun.Output[len(a.LoadRun.Output)-50000:]
			}
			if match := loadProgressRE.FindStringSubmatch(line); len(match) == 3 {
				a.LoadRun.Sent, _ = strconv.Atoi(match[1])
			}
		}
		a.LoadMu.Unlock()
	}
	err := cmd.Wait()
	now := time.Now().UTC()
	a.LoadMu.Lock()
	defer a.LoadMu.Unlock()
	if a.LoadRun == nil || a.LoadRun.ID != id {
		return
	}
	a.LoadRun.GenerationFinishedAt = &now
	persistCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer a.saveLoadRun(persistCtx, a.LoadRun)
	if ctx.Err() == context.Canceled {
		a.LoadRun.Status = "CANCELLED"
		a.LoadRun.FinishedAt = &now
		return
	}
	if err != nil {
		a.LoadRun.Error = err.Error()
	}
	if scanner.Err() != nil {
		a.LoadRun.Error = scanner.Err().Error()
	}
	a.LoadRun.Status = "DRAINING"
	if refreshErr := a.refreshLoadQueue(persistCtx, a.LoadRun); refreshErr != nil {
		a.LoadRun.Status = "FAILED"
		a.LoadRun.Error = refreshErr.Error()
	}
}

func scanLinesAndReturns(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (a *App) failLoadRun(id string, err error) {
	now := time.Now().UTC()
	a.LoadMu.Lock()
	defer a.LoadMu.Unlock()
	if a.LoadRun != nil && a.LoadRun.ID == id {
		a.LoadRun.Status = "FAILED"
		a.LoadRun.Error = fmt.Sprintf("Не удалось запустить генератор нагрузки: %v", err)
		a.LoadRun.FinishedAt = &now
		persistCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.saveLoadRun(persistCtx, a.LoadRun)
	}
}

func (a *App) handleCancelLoadTest(w http.ResponseWriter, r *http.Request) {
	a.LoadMu.Lock()
	if a.LoadRun == nil || a.LoadRun.Status != "RUNNING" {
		a.LoadMu.Unlock()
		httpx.WriteError(w, r, httpx.E(http.StatusConflict, "NO_RUNNING_LOAD_TEST", "Сейчас нет активного нагрузочного теста"))
		return
	}
	cancel := a.LoadRun.cancel
	a.LoadMu.Unlock()
	cancel()
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "CANCELLING"})
}
