global
    # Log to stdout
    log stdout format raw local0 info
    maxconn 4096

defaults
    log     global
    mode    http
    option  httplog
    option  dontlognull
    
    # Default timeouts
    timeout connect 5s
    timeout client  30s
    timeout server  30s
    # Use a long tunnel timeout to allow web sockets
    timeout tunnel  8h
    timeout http-keep-alive 2s
    
    # Default health check
    default-server inter 5s fall 1 rise 1
    
    # Enable logging of HTTP requests
    option  http-server-close
    option  forwardfor

# Stats frontend
listen stats
    bind *:8888
    mode http
    stats enable
    stats uri /
    stats refresh 5s
    stats show-legends
    stats show-node
    no log

{{ range $key, $value := .Services }}
# Frontend for {{ .Name }}
frontend {{ .Name }}
    bind *:{{ .FrontendPort }}
    mode http
    log global
    capture request header Host len 64
    use_backend be-{{ .Name }}

# Backend for {{ .Name }}
backend be-{{ .Name }}
    {{- if .HealthCheckPath }}
    option httpchk
    http-check send meth GET uri {{ .HealthCheckPath }} ver HTTP/1.1 hdr Host host.docker.internal
    {{- else }}
    option tcp-check
    {{- end }}
    {{- if gt .LocalPort 0 }}
    server local host.docker.internal:{{ .LocalPort }} check
    {{- end }}
    server k8s {{ .Name }}-srv:{{ .KubernetesPort }} check backup
{{ end }}

# mitmweb proxy - routes to mitmweb UI when available, otherwise returns a fallback page
frontend mitmweb-proxy
    bind *:8001
    default_backend mitmweb

backend mitmweb
    option tcp-check
    server mitmweb 127.0.0.1:8000 check inter 2s fall 1 rise 1
    http-error status 502 content-type "text/html; charset=utf-8" string "<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><meta name='viewport' content='width=device-width, initial-scale=1'><title>HTTP Interception Not Enabled</title><style>*{margin:0;padding:0;box-sizing:border-box}body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica Neue,sans-serif;background:#1e1e2e;color:#cdd6f4;display:flex;align-items:center;justify-content:center;min-height:100vh;padding:2rem}main{max-width:36rem;text-align:center}.icon{font-size:3rem;margin-bottom:1rem}h1{font-size:1.25rem;font-weight:600;margin-bottom:.75rem;color:#f5e0dc}p{line-height:1.6;color:#bac2de;margin-bottom:1rem}code{background:#313244;color:#a6e3a1;padding:.2rem .5rem;border-radius:4px;font-size:.9rem}.hint{font-size:.85rem;color:#a6adc8;margin-top:1.5rem}</style></head><body><main><div class='icon'>&#9675;</div><h1>HTTP Interception Not Enabled</h1><p>The mitmweb proxy is not running. Traffic is being routed directly through HAProxy without HTTP-level inspection.</p><p>To enable interception, reinstall with:</p><p><code>pilot install --intercept-http</code></p><p class='hint'>or use the short form: <code>pilot install -i</code></p></main></body></html>"
    http-error status 503 content-type "text/html; charset=utf-8" string "<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><meta name='viewport' content='width=device-width, initial-scale=1'><title>HTTP Interception Not Enabled</title><style>*{margin:0;padding:0;box-sizing:border-box}body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica Neue,sans-serif;background:#1e1e2e;color:#cdd6f4;display:flex;align-items:center;justify-content:center;min-height:100vh;padding:2rem}main{max-width:36rem;text-align:center}.icon{font-size:3rem;margin-bottom:1rem}h1{font-size:1.25rem;font-weight:600;margin-bottom:.75rem;color:#f5e0dc}p{line-height:1.6;color:#bac2de;margin-bottom:1rem}code{background:#313244;color:#a6e3a1;padding:.2rem .5rem;border-radius:4px;font-size:.9rem}.hint{font-size:.85rem;color:#a6adc8;margin-top:1.5rem}</style></head><body><main><div class='icon'>&#9675;</div><h1>HTTP Interception Not Enabled</h1><p>The mitmweb proxy is not running. Traffic is being routed directly through HAProxy without HTTP-level inspection.</p><p>To enable interception, reinstall with:</p><p><code>pilot install --intercept-http</code></p><p class='hint'>or use the short form: <code>pilot install -i</code></p></main></body></html>"
