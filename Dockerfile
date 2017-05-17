FROM alpine:3.5

EXPOSE 4000/tcp
ENV MACARON_ENV=production

# Enable https GitLab connection support
RUN apk update && \
    apk add ca-certificates curl && \
    update-ca-certificates

CMD ["/pavlik"]
COPY ./pavlik /pavlik
