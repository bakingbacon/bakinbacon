<img src="https://bakinbacon.io/img/head_logo.png" width="400px">

---

## Building BakinBacon

### Dependencies

* Install go-1.15+
* Install nodejs-14.15 (npm 6.14)

### Build Steps

1. Clone the repo
1. `cd webserver; npm install && npm run build` (Build the webserver UI; temporary step)
1. `go build` (This will download any go-lang dependencies and bundle the UI)
1. `./bakinbacon [-debug] [-trace] [-webuiaddr 127.0.0.1] [-webuiport 8082]` (Be sure to go back one directory)
1. Open http://127.0.0.1:8082/ in your browser

### Notes

* Currently hardcoded to Florencenet
