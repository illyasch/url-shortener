# Build the Go Binary.
FROM golang:1.18 as build_admin
ENV CGO_ENABLED 0
ARG BUILD_REF

# Create the admin directory and the copy the module files first and then
# download the dependencies.
RUN mkdir /admin
COPY go.* /admin/
WORKDIR /admin
RUN go mod download

# Copy the source code into the container.
COPY . /admin

# Build the admin binary. We are doing this last since this will be different
# every time we run through this process.
WORKDIR /admin/cmd/tooling/admin
RUN go build -ldflags "-X main.build=${BUILD_REF}"

# Run the Go Binary in Alpine.
FROM alpine:3.15
ARG BUILD_DATE
ARG BUILD_REF
COPY --from=build_admin /admin/cmd/tooling/admin /
WORKDIR /

LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.title="admin" \
      org.opencontainers.image.authors="Ilya Scheblanov <ilya.scheblanov@gmail.com>" \
      org.opencontainers.image.source="https://github.com/illyasch/url-shortener/cmd/admin" \
      org.opencontainers.image.revision="${BUILD_REF}"
