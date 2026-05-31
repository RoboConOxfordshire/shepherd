package gpio

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type StartHandler func(zone string)

type Button struct {
	pin     int
	arenaUSB string
	onStart  StartHandler
}

func NewButton(pin int, arenaUSBPath string, handler StartHandler) *Button {
	return &Button{
		pin:      pin,
		arenaUSB: arenaUSBPath,
		onStart:  handler,
	}
}

func (b *Button) Start() {
	go b.poll()
}

func (b *Button) poll() {
	gpioBase := "/sys/class/gpio"
	gpioName := "gpio" + strconv.Itoa(b.pin)
	gpioDir := filepath.Join(gpioBase, gpioName)

	if _, err := os.Stat(gpioDir); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(gpioBase, "export"), []byte(strconv.Itoa(b.pin)), 0644); err != nil {
			log.Printf("GPIO export failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := os.WriteFile(filepath.Join(gpioDir, "direction"), []byte("in"), 0644); err != nil {
		log.Printf("GPIO direction set failed: %v", err)
		return
	}

	valueFile := filepath.Join(gpioDir, "value")
	os.WriteFile(filepath.Join(gpioDir, "edge"), []byte("falling"), 0644)

	log.Printf("GPIO start button on pin %d (sysfs polling, falling edge)", b.pin)

	var lastState string
	for {
		data, err := os.ReadFile(valueFile)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		current := string(data)
		// Button is pulled up (1), pressed = GND (0), released = pulled up (1)
		if lastState == "1\n" && current == "0\n" {
			zone := b.detectZone()
			log.Printf("Start button pressed, zone=%s", zone)
			if b.onStart != nil {
				b.onStart(zone)
			}
			// Debounce: wait for button release
			time.Sleep(200 * time.Millisecond)
		}
		lastState = current
		time.Sleep(50 * time.Millisecond)
	}
}

func (b *Button) detectZone() string {
	for i := 1; i <= 3; i++ {
		path := filepath.Join(b.arenaUSB, fmt.Sprintf("zone%d.txt", i))
		if _, err := os.Stat(path); err == nil {
			return strconv.Itoa(i)
		}
	}
	return "0"
}
