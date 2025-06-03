FROM docker.io/library/golang:1.24.3 AS builder
COPY . /src
WORKDIR /src
ENV CGO_ENABLED=0
RUN go build -o dockyards-git -ldflags="-s -w"

FROM docker.io/library/golang:1.24.3 AS git-builder
WORKDIR /tmp
RUN apt update && apt install zlib1g-dev gettext --no-install-recommends --yes
RUN curl --silent --show-error --location https://mirrors.edge.kernel.org/pub/software/scm/git/git-2.49.0.tar.gz | tar zxpvf -
WORKDIR /tmp/git-2.49.0
RUN ./configure --prefix /tmp/static --without-tcltk CFLAGS="${CFLAGS} -static" && make install

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=git-builder /tmp/static /usr
COPY --from=builder /src/dockyards-git /usr/bin/dockyards-git
EXPOSE 9002
ENTRYPOINT ["/usr/bin/dockyards-git"]
