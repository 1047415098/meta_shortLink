#!/usr/bin/env python3
"""Verify HTTPS and a dedicated test link; remove its data only after browser checks."""
import http.cookiejar
import json
import os
import re
import urllib.error
import urllib.parse
import urllib.request

BASE = "https://meta.lu81.com"
CODE = "deploycheck0914"
UA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Version/18.0 Mobile/15E148 Safari/604.1"


class NoRedirect(urllib.request.HTTPRedirectHandler):
    # Check the WhatsApp destination without opening the external app or sending a message.
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def client():
    jar = http.cookiejar.CookieJar()
    return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar), NoRedirect()), jar


def request(opener, path, method="GET", payload=None, form=False, expected=200):
    headers = {"User-Agent": UA, "Origin": BASE, "X-Requested-With": "XMLHttpRequest"}
    data = None
    if payload is not None:
        headers["Content-Type"] = "application/x-www-form-urlencoded" if form else "application/json"
        data = (urllib.parse.urlencode(payload) if form else json.dumps(payload)).encode()
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        response = opener.open(req, timeout=25)
    except urllib.error.HTTPError as error:
        response = error
    body = response.read()
    assert response.code == expected, (path, response.code, body[:200])
    return body, response.headers


def main():
    # TLS certificates are verified by urllib's default context on every request.
    admin, _ = client()
    visitor, visitor_cookies = client()
    request(visitor, "/api/v1/meta/credentials", expected=401)
    _, headers = request(admin, "/api/v1/auth/login", "POST", {
        "username": os.environ.get("ADMIN_USER", "admin"),
        "password": os.environ.get("ADMIN_PASSWORD", "admin"),
    })
    cookie = headers.get("Set-Cookie", "")
    assert "Secure" in cookie and "HttpOnly" in cookie
    settings = json.loads(request(admin, "/api/v1/settings")[0])
    assert settings["public_base_url"] == BASE and settings["cookie_mode"] == "all"
    assert settings["geo_enabled"]
    for path in ("/admin/overview", "/admin/meta/pixels", "/admin/meta/credentials"):
        html = request(admin, path)[0].decode()
        assert 'id="app"' in html
    assets = re.findall(r'(?:src|href)="(/admin-assets/[^\"]+)"', html)
    assert assets
    for asset in set(assets):
        _, response_headers = request(admin, asset)
        assert "immutable" in response_headers.get("Cache-Control", "")

    # This isolated link has no Meta account/Pixel, so verification cannot send ad events.
    body, _ = request(admin, "/api/v1/links", "POST", {
        "code": CODE, "name": "Deployment verification - remove after testing",
        "target_url": "https://wa.me/13365661092", "enabled": True,
        "mode": "landing", "landing_brand": "Deployment verification",
        "landing_title": "Research product information",
        "landing_description": "Temporary page for deployment verification.",
        "landing_details": "This temporary test page will be removed after verification.",
        "landing_delay": 3, "attribution_mode": "dynamic", "channel": "facebook",
    })
    link = json.loads(body)
    page = "/" + CODE + "?campaign_id=101&adset_id=102&ad_id=103&site_source_name=fb"
    body, _ = request(visitor, page)
    bootstrap = json.loads(re.search(rb'<script id="landing-data"[^>]*>(.*?)</script>', body, re.S)[1])
    assert bootstrap["link"]["landing_delay"] == 3
    assert any(cookie.secure for cookie in visitor_cookies)
    ticket = bootstrap["ticket"]
    request(visitor, "/" + CODE + "/view", "POST", {"ticket": ticket}, form=True, expected=204)

    # Retry the same visit's manual and automatic events to verify separate deduplication.
    for trigger in ("manual", "manual", "auto", "auto"):
        _, response_headers = request(visitor, "/" + CODE + "/contact", "POST",
                                      {"ticket": ticket, "trigger": trigger}, form=True, expected=303)
        assert response_headers["Location"] == "https://wa.me/13365661092"
    report_path = "/api/v1/analytics?link_id=" + str(link["id"])
    summary = json.loads(request(admin, report_path)[0])["summary"]
    assert (summary["landing_views"], summary["unique"], summary["whatsapp_clicks"], summary["auto_redirects"]) == (1, 1, 1, 1), summary

    # A repeat visit keeps the visitor count; a fresh browser cookie creates another visitor.
    request(visitor, page)
    other, _ = client()
    request(other, page)
    report = json.loads(request(admin, report_path)[0])
    summary = report["summary"]
    assert (summary["landing_views"], summary["unique"], summary["whatsapp_clicks"], summary["auto_redirects"]) == (3, 2, 1, 1), summary
    assert report["health"]["write_failures"] == 0
    assert sum(row["whatsapp_clicks"] for row in report["trends"]) == 1
    assert sum(row["auto_redirects"] for row in report["trends"]) == 1
    request(admin, "/api/v1/auth/logout", "POST", {})
    print(json.dumps({"result": "passed", "test_link_id": link["id"], "test_code": CODE,
                      "summary": summary, "asset_checks": len(set(assets)), "geo_enabled": True}, ensure_ascii=False))


if __name__ == "__main__":
    main()
