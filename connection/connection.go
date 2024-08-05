package connection

type Connection interface {
    Connect() error
    Disconnect() error
    Send(data []byte) error
    Receive() ([]byte, error)
    SendStream(dataStream chan []byte) error
    ReceiveStream() (chan []byte, error)
}
