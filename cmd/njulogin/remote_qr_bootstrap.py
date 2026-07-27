import html
import json
import os
import re
import ssl
import sys
import time
import urllib.parse
import urllib.request
from http.cookiejar import CookieJar


PORTAL_BASE_URL = os.environ.get("NJU_PORTAL_BASE_URL", "https://p.nju.edu.cn").rstrip("/")
CAS_BASE_URL = os.environ.get("NJU_CAS_BASE_URL", "https://authserver.nju.edu.cn/authserver").rstrip("/")
CAS_DEST_URL = os.environ.get("NJU_CAS_DEST_URL", "https://p.nju.edu.cn:443/api/cas/getinfo/&renew=true")
TIMEOUT = int(os.environ.get("NJU_QR_TIMEOUT", "15"))
LOGIN_TIMEOUT = int(os.environ.get("NJU_QR_LOGIN_TIMEOUT", "60"))
USER_AGENT = os.environ.get(
    "NJU_USER_AGENT",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
)
VERIFY_TLS = os.environ.get("NJU_INSECURE", "0") != "1"


class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    def http_error_301(self, req, fp, code, msg, headers):
        return fp

    def http_error_302(self, req, fp, code, msg, headers):
        return fp

    def http_error_303(self, req, fp, code, msg, headers):
        return fp

    def http_error_307(self, req, fp, code, msg, headers):
        return fp

    def http_error_308(self, req, fp, code, msg, headers):
        return fp


def mask_text(value):
    value = "" if value is None else str(value)
    if not value:
        return "unknown"
    if len(value) == 1:
        return "*"
    if len(value) == 2:
        return value[:1] + "*"
    return value[:1] + ("*" * (len(value) - 2)) + value[-1:]


def mask_account(value):
    value = "" if value is None else str(value)
    if len(value) <= 6:
        return mask_text(value)
    return value[:3] + "***" + value[-3:]


def mask_mac(value):
    value = "" if value is None else str(value)
    parts = value.split(":")
    if len(parts) != 6:
        return "unknown" if not value else "redacted"
    return ":".join(parts[:2] + ["xx", "xx", "xx"] + parts[-1:])


def format_time(value):
    try:
        return time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(int(value)))
    except Exception:
        return "unknown"


def input_value(document, name):
    pattern = re.compile(r"(?is)<input[^>]+name=[\"']%s[\"'][^>]*>" % re.escape(name))
    value_pattern = re.compile(r"(?is)value=[\"']([^\"']*)[\"']")
    tag = pattern.search(document)
    if not tag:
        return ""
    value = value_pattern.search(tag.group(0))
    if not value:
        return ""
    return html.unescape(value.group(1))


def request(opener, url, data=None, content_type=None):
    headers = {"User-Agent": USER_AGENT}
    if content_type:
        headers["Content-Type"] = content_type
    req = urllib.request.Request(url, data=data, headers=headers)
    with opener.open(req, timeout=TIMEOUT) as resp:
        return resp.read().decode("utf-8", errors="replace")


def main():
    jar = CookieJar()
    handlers = [urllib.request.HTTPCookieProcessor(jar)]
    if VERIFY_TLS:
        opener = urllib.request.build_opener(*handlers)
        opener_no_redirect = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar), NoRedirectHandler())
    else:
        context = ssl._create_unverified_context()
        opener = urllib.request.build_opener(urllib.request.HTTPSHandler(context=context), *handlers)
        opener_no_redirect = urllib.request.build_opener(
            urllib.request.HTTPSHandler(context=context),
            urllib.request.HTTPCookieProcessor(jar),
            NoRedirectHandler(),
        )

    login_url = CAS_BASE_URL + "/login?service=" + urllib.parse.quote(CAS_DEST_URL, safe="")
    login_page = request(opener, login_url)
    fields = {
        "lt": input_value(login_page, "lt"),
        "dllt": input_value(login_page, "dllt"),
        "execution": input_value(login_page, "execution"),
        "_eventId": input_value(login_page, "_eventId"),
        "rmShown": input_value(login_page, "rmShown"),
    }

    token = request(opener, CAS_BASE_URL + "/qrCode/getToken?ts=%d" % int(time.time() * 1000)).strip()
    if not token:
        raise RuntimeError("empty QR token")

    qr_url = CAS_BASE_URL + "/qrCode/qrCodeLogin.do?uuid=" + urllib.parse.quote(token, safe="")
    print("QR_URL=" + qr_url, flush=True)
    print("scan_state=waiting", flush=True)

    confirmed_reported = False
    deadline = time.time() + LOGIN_TIMEOUT
    while time.time() < deadline:
        status_url = CAS_BASE_URL + "/qrCode/getStatus.htl?ts=%d&uuid=%s" % (
            int(time.time() * 1000),
            urllib.parse.quote(token, safe=""),
        )
        status = request(opener, status_url).strip()
        if status == "1":
            break
        if status == "2" and not confirmed_reported:
            print("scan_state=confirmed_on_phone_pending", flush=True)
            confirmed_reported = True
        if status == "3":
            raise RuntimeError("QR code expired")
        time.sleep(1)
    else:
        raise RuntimeError("QR login timed out")

    form = urllib.parse.urlencode(
        {
            "lt": fields["lt"],
            "uuid": token,
            "cllt": "qrLogin",
            "dllt": fields["dllt"],
            "execution": fields["execution"],
            "_eventId": fields["_eventId"],
            "rmShown": fields["rmShown"],
        }
    ).encode("utf-8")
    request(opener_no_redirect, login_url, data=form, content_type="application/x-www-form-urlencoded")
    print("state=qr_login_submitted", flush=True)

    info = request(opener, PORTAL_BASE_URL + "/api/portal/v1/getinfo?_=%d" % int(time.time() * 1000))
    response = json.loads(info)
    print("reply_code=%s" % response.get("reply_code"), flush=True)
    if response.get("reply_msg"):
        print("reply_msg=%s" % response["reply_msg"], flush=True)
    rows = ((response.get("results") or {}).get("rows") or [])
    if response.get("reply_code") == 0 and rows:
        print("state=authenticated", flush=True)
        print("sessions=%d" % len(rows), flush=True)
        for index, row in enumerate(rows, start=1):
            print(
                "session_%d=%s %s start=%s service=%s mac=%s ip=redacted"
                % (
                    index,
                    mask_account(row.get("username")),
                    mask_text(row.get("fullname")),
                    format_time(row.get("acctstarttime")),
                    row.get("service_name") or "unknown",
                    mask_mac(row.get("mac")),
                ),
                flush=True,
            )
    else:
        print("state=not_authenticated", flush=True)


if __name__ == "__main__":
    main()
