<img src="https://bakinbacon.io/img/head_logo.png" width="400px">

---

## Building BakinBacon

* Install go-1.15+
* Install nodejs-14.15 (npm 6.14)
* Clone the repo
* `go build` (This will download any dependencies)
* `cd webserver; npm run build` (Build the webserver UI; temporary step)
* `./bakinbacon [-debug] [-trace] [-webuiaddr 127.0.0.1] [-webuiport 8082]` (Be sure to go back one directory)
* Open http://127.0.0.1:8082/ in your browser

### Notes

* Currently hardcoded to Florencenet
