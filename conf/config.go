package conf

type Database struct {
	Type        string `json:"type"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Name        string `json:"name"`
	TablePrefix string `json:"table_prefix"`
	DBFile      string `json:"db_file"`
}

type Scheme struct {
	Https    bool   `json:"https"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type CacheConfig struct {
	Expiration      int64 `json:"expiration"`
	CleanupInterval int64 `json:"cleanup_interval"`
}

type Config struct {
	Address  string      `json:"address"`
	Port     int         `json:"port"`
	Assets   string      `json:"assets"`
	Database Database    `json:"database"`
	Scheme   Scheme      `json:"scheme"`
	Cache    CacheConfig `json:"cache"`
}

func DefaultConfig() *Config {
	return &Config{
		Address: "0.0.0.0",
		Port:    5244,
		Assets:  "zhimg",
		Database: Database{
			Type:        "sqlite3",
			Port:        0,
			TablePrefix: "x_",
			DBFile:      "data/data.db",
		},
		Cache: CacheConfig{
			Expiration:      60,
			CleanupInterval: 120,
		},
	}
}
