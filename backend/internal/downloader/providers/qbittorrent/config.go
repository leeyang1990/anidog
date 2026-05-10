package qbittorrent

type Config struct {
	Host     string
	Username string
	Password string
}

func NewConfig(host, username, password string) *Config {
	return &Config{
		Host:     host,
		Username: username,
		Password: password,
	}
}
