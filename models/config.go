package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sublink/utils"

	"gopkg.in/yaml.v3"
)

type Config struct {
	JwtSecret  string `yaml:"jwt_secret"`
	ExpireDays int    `yaml:"expire_days"`
	Port       int    `yaml:"port"`
}

var comment string = `# jwt_secret: JWT secret
# expire_days: token expiration days
# port: listen port
`

func ConfigInit() {
	if err := os.MkdirAll(filepath.Dir("./db/config.yaml"), 0755); err != nil {
		log.Println("create config directory failed:", err)
		return
	}

	if _, err := os.Stat("./db/config.yaml"); os.IsNotExist(err) {
		defaultConfig := Config{
			JwtSecret:  utils.RandString(31),
			ExpireDays: 14,
			Port:       8000,
		}

		data, err := yaml.Marshal(&defaultConfig)
		if err != nil {
			log.Println("marshal default config failed:", err)
			return
		}

		data = []byte(comment + string(data))
		err = os.WriteFile("./db/config.yaml", data, 0644)
		if err != nil {
			fmt.Println("write config file failed:", err)
			return
		}
		log.Println("config file created")
	}
}

func ReadConfig() Config {
	file, err := os.ReadFile("./db/config.yaml")
	if err != nil {
		log.Println(err)
	}
	cfg := Config{}
	yaml.Unmarshal(file, &cfg)
	return cfg
}

func SetConfig(newCfg Config) {
	oldCfg := ReadConfig()
	if newCfg.JwtSecret != "" {
		oldCfg.JwtSecret = newCfg.JwtSecret
	}
	if newCfg.ExpireDays != 0 {
		oldCfg.ExpireDays = newCfg.ExpireDays
	}
	if newCfg.Port != 0 {
		oldCfg.Port = newCfg.Port
	}

	data, err := yaml.Marshal(&oldCfg)
	if err != nil {
		log.Println(err)
	}
	data = []byte(comment + string(data))
	os.WriteFile("./db/config.yaml", data, 0644)
}
