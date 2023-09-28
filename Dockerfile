FROM golang:1.19-alpine

RUN apk add --update --no-cache \
    curl \
    py-pip \
    build-base \
    git \
  && pip install awscli

WORKDIR /go/src/github.com/keikoproj/cluster-validator
COPY . .
RUN git rev-parse HEAD
RUN date +%FT%T%z
RUN make build
RUN cp ./bin/cluster-validator /bin/cluster-validator \
    && chmod +x /bin/cluster-validator
ENV HOME /root

ENTRYPOINT ["/bin/cluster-validator"]
CMD ["help"]
