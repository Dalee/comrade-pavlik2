FROM alpine:3.8

EXPOSE 4000/tcp
ENV MACARON_ENV=production

# Enable https GitLab connection support
RUN apk add --no-cache ca-certificates curl && \
    update-ca-certificates

CMD ["/pavlik"]
COPY ./pavlik /pavlik
