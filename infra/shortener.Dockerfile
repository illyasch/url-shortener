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
RUN go build -ldflags "-X main.build=${BUILD_REF}"

# Run the Go Binary in Alpine.
FROM alpine:3.15
ARG BUILD_DATE
ARG BUILD_REF
RUN addgroup -g 1000 -S shortener && \
    adduser -u 1000 -h /shortener -G shortener -S shortener
COPY --from=build_shortener --chown=shortener:shortener /shortener/cmd/url-shortener/url-shortener /shortener/url-shortener
WORKDIR /shortener
USER shortener
CMD ["./url-shortener"]

LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.title="url-shortener" \
      org.opencontainers.image.authors="Ilya Scheblanov <ilya.scheblanov@gmail.com>" \
      org.opencontainers.image.source="https://github.com/illyasch/url-shortener/cmd/url-shortener" \
      org.opencontainers.image.revision="${BUILD_REF}"
