FROM golang:latest

WORKDIR /go/github.com/yunxiaozhao/PBFT

ADD . /go/github.com/yunxiaozhao/PBFT

RUN go env -w GOPROXY=https://goproxy.cn\
 && go build .
 
CMD ["./consensusPBFT 0x01"]

