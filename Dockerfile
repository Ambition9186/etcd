FROM golang
ADD . /go/src/github.com/ambition9186/etcd
ADD cmd/vendor /go/src/github.com/ambition9186/etcd/vendor
RUN go install github.com/ambition9186/etcd
EXPOSE 2379 2380
ENTRYPOINT ["etcd"]
