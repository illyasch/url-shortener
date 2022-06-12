# Build the Go Binary.
FROM golang:1.18 as build_shortener
ENV CGO_ENABLED 0
ARG BUILD_REF

# Create the shortener directory and the copy the module files first and then
# download the dependencies.
RUN mkdir /shortener
COPY go.* /shortener/
WORKDIR /shortener
RUN go mod download

# Copy the source code into the container.
COPY . /shortener

# Build the shortener binary. We are doing this last since this will be different
# every time we run through this process.
WORKDIR /shortener/cmd/url-shortener
CMD CGO_ENABLED=1 go test -count=1 -short -race ./...
