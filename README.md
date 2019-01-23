# homie-ota-deploy

This is a tool to upload firmware to a device running esp32-homie

## Install

`go get github.com/craftmetrics/homie-ota-deploy`

## Usage

```
homie-ota-deploy \
    -bin build/my-firmware.bin \
    -broker tcp://mqtt.example.com:1883 \
    -device homie/mydevice \
    -username admin \
    -password adminpass
```

## Known Issues

On a mac, the built-in firewall will ask each time if you wish to allow incoming connections. To permanently accept that, see https://stackoverflow.com/a/21052254
