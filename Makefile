container_runtime := $(shell which docker || which podman )

$(info using ${container_runtime})

up: down
	${container_runtime} compose up --build -d

down:
	${container_runtime} compose down

clean:
	${container_runtime} compose down -v

run-tests: 
	${container_runtime} run --rm --network=host tests:latest

test:
	make clean
	make up
	@echo wait cluster to start && sleep 10
	make run-tests
	make clean
	@echo "test finished"

lint:
	make -C search-services lint

proto:
	make -C search-services protobuf

unit:
	make -C search-services test
	mv search-services/cover.html .
fmt: 
	make -C search-services fmt
mockgen:
	make -C search-services mockgen
tools:
	go install github.com/yoheimuta/protolint/cmd/protolint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.4.0
	@echo "checking protobuf compiler, if it fails follow guide at https://protobuf.dev/installation/"
	@which -s protoc && echo OK || exit 1

install-repos:
	@echo "Adding Helm repositories..."
	helm repo add victoria-metrics https://victoriametrics.github.io/helm-charts/
	helm repo add grafana https://grafana.github.io/helm-charts
	helm repo update

install-infra:
	@echo "Installing VictoriaMetrics Stack..."
	helm upgrade --install vm victoria-metrics/victoria-metrics-k8s-stack \
		--set victoria-metrics-operator.enabled=true \
		--rollback-on-failure

	@echo "Installing Loki Stack..."
	helm upgrade --install loki grafana/loki-stack \
		--set loki.persistence.enabled=true \
		--set loki.persistence.size=1Gi \
		--set promtail.enabled=true \
		--set grafana.sidecar.datasources.isDefault=false \
		--rollback-on-failure

clean-infra:
	helm uninstall vm || true
	helm uninstall loki || true
	kubectl delete pvc -l app.kubernetes.io/instance=vm || true
	kubectl delete pvc -l app.kubernetes.io/instance=loki || true

build-deps:
	@echo "Building dependencies for search-app-chart..."
	helm dependency build ./search-app-chart

minikube-up:
	minikube start --memory=4192 --cpus=4
	skaffold dev

minikube-setup: install-repos install-infra build-deps
	@echo "Infrastructure is ready!"

minikube-test:
	@echo "Building tests image..."
	eval $$(minikube docker-env) && docker build -t tests:local ./tests
	@echo "Cleaning up old jobs..."
	kubectl delete job search-app-tests --ignore-not-found
	@echo "Starting tests in K8s..."
	kubectl apply -f k8s-test-job.yaml
	@echo "Waiting for pod to start..."
	sleep 3
	kubectl logs -f job/search-app-tests
	@echo "Checking final status..."
	kubectl wait --for=condition=complete job/search-app-tests --timeout=600s