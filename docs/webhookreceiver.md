# Webhook Receiver


## AMReceiver handler
`AMReceiver` handler exposes api path defined in `AMReceiverPath`. It expects POST requests from alert manager containing `AMReceiverData` which is defined in [template.Data](https://pkg.go.dev/github.com/prometheus/alertmanager@v0.21.0/template#Data). `processAMReceiver` worker is used to process `AMReceiverData`. The endpoint responds with data structure defined in `AMReceiverResponse`.

To test using curl use:
```
curl -X POST http://<server>/alertmanager-receiver -H 'Content-Type: application/json' -d '{"status":"...","receiver":"..."}'
```

