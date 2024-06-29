FROM golang:alpine
WORKDIR /go/src/app
COPY . .
RUN go mod tidy
RUN go build -o app
EXPOSE 8080
CMD ["./app"]
LABEL community="Uniworkhub"
LABEL maintainer="Ekrem Sekmen(esekmen), Furkan Çiftçi(stok), Berat Yazır(byazir)"
LABEL description="This is a Docker image for our Golang ascii-web application running locally on localhost:8080."
LABEL version="0.0.1"
LABEL environment="Development"
LABEL license="MIT"
LABEL url="http://localhost:8080/"
