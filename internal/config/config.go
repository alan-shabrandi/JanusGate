package config

type Config struct {
	Port string
	Env  string
}

func LoadConfig() (*Config, error) {
	return &Config{
		Port: "8080",
		Env:  "development",
	}, nil
}
