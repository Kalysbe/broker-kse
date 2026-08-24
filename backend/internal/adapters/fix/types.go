package fix

// Canonical FIX application messages. Vendor codecs (ARQA, QuickFIX/J bridges, etc.)
// implement Codec; OMS never depends on tag dictionaries.

type MessageType string

const (
	Heartbeat                 MessageType = "0"
	Logon                     MessageType = "A"
	Logout                    MessageType = "5"
	Reject                    MessageType = "3"
	NewOrderSingle            MessageType = "D"
	OrderCancelRequest        MessageType = "F"
	OrderCancelReplaceRequest MessageType = "G"
	ExecutionReport           MessageType = "8"
	OrderCancelReject         MessageType = "9"
)

type Message struct {
	Type   MessageType
	Fields map[int]string
}

type Codec interface {
	Encode(Message) ([]byte, error)
	Decode([]byte) (Message, error)
}

type Session interface {
	Logon() error
	Logout() error
	Send(Message) error
	Health() string
}
