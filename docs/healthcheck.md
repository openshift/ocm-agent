# Health Check


## Livez handler
`Livez` handler exposes api path defined in `LivezPath`. It expects GET requests. The endpoint responds with data structure defined in `LivezResponse`.

To test using curl use:
```
curl http://<server>/livez
```

## Readyz handler
`Readyz` handler exposes api path defined in `ReadyzPath`. It expects GET requests. The endpoint responds with data structure defined in `ReadyResponse`.

To test using curl use:
```
curl http://<server>/readyz
```
