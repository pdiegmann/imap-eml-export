FROM debian:bookworm-slim

RUN apt-get update -qq \
    && apt-get install -y --no-install-recommends dovecot-imapd openssl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r vmail \
    && useradd -r -g vmail -d /var/mail -s /sbin/nologin vmail

COPY dovecot.conf /etc/dovecot/dovecot.conf
COPY sample-emails/ /sample-emails/
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 143 993

ENTRYPOINT ["/entrypoint.sh"]
