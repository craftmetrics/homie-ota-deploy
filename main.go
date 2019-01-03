package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var listen = flag.String("listen", ":8081", "ip:port for http server to listen on")
var binPath = flag.String("bin", "", "path to build product")
var mqttDeviceTopic = flag.String("device", "homie/mydevice", "MQTT topic for device")
var mqttBroker = flag.String("broker", "tcp://data.craftmetrics.ca:1883", "MQTT broker URI")
var mqttUsername = flag.String("username", "test", "MQTT username")
var mqttPassword = flag.String("password", "test", "MQTT password")

var triggered = false
var complete = make(chan (struct{}))

// resolveHostIP attempts to get an interface address suitable
// for our web server to bind to
func resolveHostIP() string {
	netInterfaceAddresses, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, netInterfaceAddress := range netInterfaceAddresses {
		networkIP, ok := netInterfaceAddress.(*net.IPNet)
		if ok && !networkIP.IP.IsLoopback() && networkIP.IP.To4() != nil {
			ip := networkIP.IP.String()
			return ip
		}
	}
	return ""
}

func handleMessage(client mqtt.Client, msg mqtt.Message) {
	if strings.HasSuffix(msg.Topic(), "ota/status") {
		// Suppress messages from before we begin
		if !triggered {
			return
		}

		status := strings.SplitN(string(msg.Payload()), " ", 2)
		if status[0] == "200" {
			log.Printf("OTA Success")
			complete <- struct{}{}
		} else if status[0] == "206" {
			log.Printf("wrote %s bytes", status[1])
		} else {
			log.Printf("OTA Failure: %s", msg.Payload())
			complete <- struct{}{}
		}
	} else {
		fmt.Printf("[%v,%v,%v] %s: %s (%d)\n", msg.Qos(), msg.Retained(), msg.Duplicate(), msg.Topic(), msg.Payload(), msg.MessageID())
	}
}

func main() {
	flag.Parse()

	serverAddr := *listen
	if strings.HasPrefix(serverAddr, ":") {
		serverAddr = resolveHostIP() + serverAddr
	}
	log.Printf("Launching web server on %s", serverAddr)

	// Start web server
	http.HandleFunc("/"+*binPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, *binPath)
	})
	go func() {
		panic(http.ListenAndServe(serverAddr, nil))
	}()

	// Connect to mqtt
	opts := mqtt.NewClientOptions()
	opts.AddBroker(*mqttBroker)
	opts.SetClientID("homie-ota-deploy")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetDefaultPublishHandler(handleMessage)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetUsername(*mqttUsername)
	opts.SetPassword(*mqttPassword)
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	defer c.Disconnect(250)

	// Subscribe to relevent OTA responses
	if token := c.Subscribe(*mqttDeviceTopic+"/$implementation/ota/#", 0, nil); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// Send the OTA trigger
	url := "http://" + serverAddr + "/" + *binPath
	log.Printf("Triggering OTA update from %s", url)
	if token := c.Publish(*mqttDeviceTopic+"/$implementation/ota/url", 2, false, url); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	triggered = true

	// Wait until complete
	select {
	case <-complete:
	}
}
