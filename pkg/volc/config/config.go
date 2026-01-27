package config

import (
	"os"
)

func AppKey() string {
	return os.Getenv("VOLC_APPID")
}

func AccessKey() string {
	return os.Getenv("VOLC_TOKEN")
}
