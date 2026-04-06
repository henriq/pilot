apiVersion: apps/v1
kind: Deployment
metadata:
  name: dev-proxy
  labels:
    app: dev-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: dev-proxy
  template:
    metadata:
      labels:
        app: dev-proxy
      annotations:
        checksum: {{ .Checksum }}
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: haproxy
        image: {{ .ImagePrefix }}/haproxy-{{ .Name }}
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
        - containerPort: 8888
        - containerPort: 8001
        securityContext:
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: haproxy-data
          mountPath: /var/lib/haproxy
{{- if .InterceptHttp }}
      - name: mitmproxy
        image: {{ .ImagePrefix }}/mitmproxy-{{ .Name }}
        imagePullPolicy: Never
        tty: true
        stdin: true
        securityContext:
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: mitmproxy-cache
          mountPath: /home/nonroot/.cache
        - name: mitmproxy-data
          mountPath: /data
        - name: mitmproxy-home
          mountPath: /home/nonroot/.mitmproxy
        command: ["mitmweb"]
        args:
          - --set
          - keep_host_header=true
          - --set
          - web_password={{ .Password }}
          - --set
          - onboarding=false
          - --set
          - web_open_browser=false
          - --set
          - showhost=true
          - --web-host=0.0.0.0
          - --web-port=8000
{{- range $key, $value := .Services }}
          - --mode=reverse:http://localhost:{{.FrontendPort}}@{{ .ProxyPort }}
{{- end }}
{{- end }}
      volumes:
      - name: tmp
        emptyDir:
          medium: Memory
      - name: haproxy-data
        emptyDir: {}
{{- if .InterceptHttp }}
      - name: mitmproxy-cache
        emptyDir:
          medium: Memory
      - name: mitmproxy-data
        emptyDir: {}
      - name: mitmproxy-home
        emptyDir: {}
{{- end }}

---

apiVersion: v1
kind: Service
metadata:
  name: dev-proxy
spec:
  selector:
    app: dev-proxy
  ports:
  - protocol: TCP
    port: 8888
    targetPort: 8888
    name: stats
  - protocol: TCP
    port: 8001
    targetPort: 8001
    name: mitmweb-proxy

---

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dev-proxy-haproxy
spec:
  tls:
  - hosts:
    - stats.dev-proxy.{{ .Name }}.localhost
    secretName: {{ .TLSSecretName }}
  rules:
  - host: stats.dev-proxy.{{ .Name }}.localhost
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: dev-proxy
            port:
              number: 8888

---

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dev-proxy-mitmweb
spec:
  tls:
  - hosts:
    - dev-proxy.{{ .Name }}.localhost
    secretName: {{ .TLSSecretName }}
  rules:
  - host: dev-proxy.{{ .Name }}.localhost
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: dev-proxy
            port:
              number: 8001

---

{{- range $key, $value := .Services }}

apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-srv
spec:
  selector:
{{ .Selector | toYaml | indent 4}}
  ports:
  - protocol: TCP
    port: {{ .KubernetesPort }}
    targetPort: {{ .KubernetesPort }}
    name: http

---

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Name }}-dx
spec:
  tls:
  - hosts:
    - {{ .Name }}.{{ $.Name }}.localhost
    secretName: {{ $.TLSSecretName }}
  rules:
  - host: {{ .Name }}.{{ $.Name }}.localhost
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: {{ .Name }}
            port:
              number: {{ .KubernetesPort }}

---

{{- end}}
