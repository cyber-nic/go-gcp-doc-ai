FROM golang:1.21-alpine as build
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /code
COPY ./apps/ocr-worker/go.mod ./apps/ocr-worker/go.sum ./
RUN go mod download

COPY ./apps/ocr-worker ./apps/ocr-worker
COPY ./libs ./libs
RUN CGO_ENABLED=0 go build -o ./bin/app ./apps/ocr-worker/*.go

FROM scratch
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /code/bin/app /app
USER appuser
CMD ["/app"]