## About
A deployment controller that notifies when a deployment object is created/deleted.

### How to run
Copy all third-party dependencies to a vendor folder in your project root.
```
go mod vendor
```
Create a deployment.
```
kubectl apply -f samples/deploy.yml
```
Run the controller from the host.
```
go run main.go
```