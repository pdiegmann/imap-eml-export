#!/bin/sh
set -e

MAIL_USER="${MAIL_USER:-testuser}"
MAIL_PASS="${MAIL_PASS:-testpassword}"
MAIL_DIR="/var/mail/${MAIL_USER}/Maildir"

# ── Credentials ──────────────────────────────────────────────────────────────
printf '%s:{PLAIN}%s\n' "$MAIL_USER" "$MAIL_PASS" > /etc/dovecot/users

# ── TLS certificate (self-signed) ─────────────────────────────────────────────
if [ ! -f /etc/dovecot/ssl/dovecot.pem ]; then
    mkdir -p /etc/dovecot/ssl
    openssl req -new -x509 -days 3650 -nodes \
        -out /etc/dovecot/ssl/dovecot.pem \
        -keyout /etc/dovecot/ssl/dovecot.key \
        -subj "/CN=localhost" 2>/dev/null
    chmod 600 /etc/dovecot/ssl/dovecot.key
fi

# ── Maildir structure ─────────────────────────────────────────────────────────
# Always recreate from sample emails so the test state is deterministic.
rm -rf "${MAIL_DIR}"

for folder in cur new tmp \
              .Sent/cur .Sent/new .Sent/tmp \
              .Work/cur .Work/new .Work/tmp \
              ".Work.ProjectA/cur" ".Work.ProjectA/new" ".Work.ProjectA/tmp" \
              .Drafts/cur .Drafts/new .Drafts/tmp; do
    mkdir -p "${MAIL_DIR}/${folder}"
done

# Write subscriptions file so all folders appear via LIST
cat > "${MAIL_DIR}/subscriptions" <<'EOF'
INBOX
Sent
Work
Work/ProjectA
Drafts
EOF

# ── Populate sample emails ────────────────────────────────────────────────────
i=1
for f in /sample-emails/inbox-*.eml; do
    [ -f "$f" ] || continue
    # Use a fixed base timestamp + counter for reproducible filenames
    cp "$f" "${MAIL_DIR}/new/1705312800.M${i}P1.localhost"
    i=$((i + 1))
done

i=1
for f in /sample-emails/sent-*.eml; do
    [ -f "$f" ] || continue
    cp "$f" "${MAIL_DIR}/.Sent/cur/1705399200.M${i}P1.localhost:2,S"
    i=$((i + 1))
done

i=1
for f in /sample-emails/work-*.eml; do
    [ -f "$f" ] || continue
    cp "$f" "${MAIL_DIR}/.Work/cur/1705485600.M${i}P1.localhost:2,S"
    i=$((i + 1))
done

i=1
for f in /sample-emails/project-*.eml; do
    [ -f "$f" ] || continue
    cp "$f" "${MAIL_DIR}/.Work.ProjectA/cur/1705572000.M${i}P1.localhost:2,S"
    i=$((i + 1))
done

# ── Permissions ──────────────────────────────────────────────────────────────
chown -R vmail:vmail "/var/mail/${MAIL_USER}"

echo "IMAP server ready — user: ${MAIL_USER}  plain-IMAP: 143  IMAPS: 993"

# ── Start Dovecot in foreground ───────────────────────────────────────────────
exec dovecot -F
