.PHONY: cluster docker-build kind-load deploy clean

cluster:
	kind create cluster --config=kind/kind-config.yaml --name=antrea-capture
	helm repo add antrea https://charts.antrea.io
	helm repo update
	helm install antrea antrea/antrea -n kube-system --wait

cluster-delete:
	kind delete cluster --name=antrea-capture

docker-build:
	docker build -t capture-controller:latest .

kind-load:
	kind load docker-image capture-controller:latest --name antrea-capture

deploy:
	kubectl apply -f manifests/daemonset.yaml
	kubectl apply -f manifests/test-pod.yaml

undeploy:
	kubectl delete -f manifests/test-pod.yaml --ignore-not-found
	kubectl delete -f manifests/daemonset.yaml --ignore-not-found

collect-outputs:
	mkdir -p output
	kubectl describe pod test-pod > output/pod-describe.txt
	kubectl get pods -A > output/pods.txt
	kubectl exec -n kube-system $$(kubectl get pods -n kube-system -l app=capture-controller -o jsonpath='{.items[0].metadata.name}') -- ls -l /tmp/capture-* > output/capture-files.txt
	kubectl cp kube-system/$$(kubectl get pods -n kube-system -l app=capture-controller -o jsonpath='{.items[0].metadata.name}'):/tmp/capture-test-pod.pcap0 output/capture.pcap
	kubectl exec -n kube-system $$(kubectl get pods -n kube-system -l app=capture-controller -o jsonpath='{.items[0].metadata.name}') -- tcpdump -r /tmp/capture-test-pod.pcap0 > output/capture-output.txt 2>&1

clean:
	rm -rf output/
