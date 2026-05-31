package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Port              int
	UserCodePath      string
	UserCodeEntrypoint string
	RobotLibPath      string
	OutputFilePath    string
	CameraImagePath   string
	ArenaUSBPath      string
	TeamNameFilePath  string
	GameLogoPath      string
	RoundLength       time.Duration
	ReapGracePeriod   time.Duration
	StartButtonPin    int
	ImageWidth        float64
	ImageHeight       float64
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDur(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func Load() *Config {
	home := getEnv("HOME", "/home/pi")
	wd, _ := os.Getwd()

	return &Config{
		Port:              getEnvInt("SHEPHERD_PORT", 80),
		UserCodePath:      getEnv("SHEPHERD_USER_CODE_PATH", filepath.Join(wd, "usercode")),
		UserCodeEntrypoint: "main.py",
		RobotLibPath:      getEnv("SHEPHERD_ROBOT_LIB_PATH", filepath.Join(home, "robot")),
		OutputFilePath:    getEnv("SHEPHERD_OUTPUT_FILE", "/media/RobotUSB/logs.txt"),
		CameraImagePath:   getEnv("SHEPHERD_CAMERA_IMAGE", filepath.Join(wd, "shepherd", "static", "image.jpg")),
		ArenaUSBPath:      getEnv("SHEPHERD_ARENA_USB_PATH", "/media/ArenaUSB"),
		TeamNameFilePath:  getEnv("SHEPHERD_TEAM_NAME_FILE", filepath.Join(home, "teamname.txt")),
		GameLogoPath:      getEnv("SHEPHERD_GAME_LOGO_PATH", filepath.Join(home, "game_logo.jpg")),
		RoundLength:       getEnvDur("SHEPHERD_ROUND_LENGTH", 180*time.Second),
		ReapGracePeriod:   getEnvDur("SHEPHERD_REAP_GRACE", 5*time.Second),
		StartButtonPin:    getEnvInt("SHEPHERD_START_BUTTON_PIN", 26),
		ImageWidth:        float64(getEnvInt("SHEPHERD_IMAGE_WIDTH", 800)),
		ImageHeight:       float64(getEnvInt("SHEPHERD_IMAGE_HEIGHT", 600)),
	}
}
