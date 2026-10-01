.PHONY: tools test integration analysis-test build manifests bootstrap status smoke load canary-traffic failure-pod failure-database-pod failure-worker-node policy-test drift-demo canary-good canary-fail canary-restore grafana-ui argocd-ui argocd-password teardown
tools:
	./scripts/tools.sh
test:
	go test -race ./...
	go vet ./...
	npm test --prefix web
	npm run build --prefix web
integration:
	./scripts/integration.sh
analysis-test:
	./scripts/test-analysis.sh
build:
	docker build -t ticket-platform-backend:local .
	docker build -t ticket-platform-web:local web
manifests:
	python3 scripts/render-workloads.py
	python3 scripts/render-platform.py
	python3 scripts/render-dashboard.py
	./scripts/validate.sh
bootstrap:
	./scripts/bootstrap.sh
status:
	./scripts/status.sh
smoke:
	./scripts/demo.sh smoke
load:
	./scripts/demo.sh load
canary-traffic:
	./scripts/demo.sh canary-traffic
failure-pod:
	./scripts/demo.sh failure-pod
failure-database-pod:
	./scripts/demo.sh failure-database-pod
failure-worker-node:
	./scripts/demo.sh failure-worker-node
policy-test:
	./scripts/demo.sh policy-test
drift-demo:
	./scripts/demo.sh drift-demo
canary-fail:
	./scripts/demo.sh canary-fail
canary-good:
	./scripts/demo.sh canary-good
canary-restore:
	./scripts/demo.sh canary-restore
grafana-ui:
	KUBECONFIG="$(CURDIR)/.local/kubeconfig" kubectl --context kind-ticket-platform -n monitoring port-forward svc/monitoring-grafana 3000:80
argocd-ui:
	KUBECONFIG="$(CURDIR)/.local/kubeconfig" kubectl --context kind-ticket-platform -n argocd port-forward svc/argocd-server 8085:80
argocd-password:
	@KUBECONFIG="$(CURDIR)/.local/kubeconfig" kubectl --context kind-ticket-platform -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
	@printf '\n'
teardown:
	./scripts/teardown.sh
