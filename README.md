# url-shortener

This service reduces long links from sites on the Internet. If you like the project or use it for any purpose, don't hesitate to give it a star on GitHub!

## Overview

The entry point to the code is in cmd/url-shortener/url-shortener.go. The service has the following HTTP handlers:

- _/shorten_ - use the POST method and x-www-form-urlencoded parameters url with a URL for shortening.
  Returns base62 code of the URL.
- _/{code}_ - use GET method and substitute {code} with actual URL code received from the service like **udXWFB**. Returns the full URL from the code.
- _/readiness_ - check if the database is ready and, if not, will return a 500 status.
- _/liveness_ - return simple status info if the service is alive.

## Prerequisites

- [Docker](https://www.docker.com/) and [docker-compose](https://docs.docker.com/compose/install/)
- [Git](https://git-scm.com/)
- [GNU Make](https://www.gnu.org/software/make/)
- [Go](https://golang.org/) (if you want to compile and run the service without docker)

## Installation

1. Clone this repository in the current directory:

   ```
   git clone https://github.com/illyasch/url-shortener
   ```

2. Build Docker images:

   ```bash
   make image
   ```

3. Migrate and seed the database (uses Docker):

   ```
   make seed
   ```

3. Start the local development environment (uses Docker):

   ```
   make up
   ```

   At this point, you should have the url-shortener service running. To confirm the state of the running Docker container, run

   ```
   $ docker ps
   ```

## How to

### Run unit tests

from the docker container

```
make test
```

### Run manual tests

Shorten a URL
   ```
   $ curl -i --data-urlencode "url=http://www.cnn.com" http://localhost:3000/shorten
   HTTP/1.1 200 OK
   Content-Type: application/json
   Date: Sun, 12 Jun 2022 16:05:58 GMT
   Content-Length: 17
   
   {"code":"vdXWFB"}
   ```

Get a shortened URL with the code.
   ```
   $ curl -i http://localhost:3000/vdXWFB
   HTTP/1.1 200 OK
   Content-Type: application/json
   Date: Sun, 12 Jun 2022 16:07:48 GMT
   Content-Length: 28
   
   {"url":"http://www.cnn.com"}
   ```