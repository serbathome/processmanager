FROM golang:1.23 as builder
WORKDIR /app
COPY . .
RUN ./build.sh
FROM alpine:3.21
RUN apk add --no-cache libc6-compat
COPY --from=builder /app/pm /pm
COPY --from=builder /app/aefuf /aefuf
COPY --from=builder /app/aefad /aefad
COPY --from=builder /app/config.json /config.json
CMD ["/pm"]
