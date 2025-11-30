package grpc

type Config struct {
	BindIP      string
	Port        uint16
	TLSCertFile string
	TLSKeyFile  string
}
