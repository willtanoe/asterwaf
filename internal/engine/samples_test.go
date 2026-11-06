package 引擎

import "testing"

type 规则样本 struct {
	恶意 []string
	正常 []string
}

func 默认规则样本() map[string]规则样本 {
	return map[string]规则样本{
		"sqli.union-select": {
			恶意: []string{"1 UNION ALL SELECT username, password FROM users", "' union select 1,2--"},
			正常: []string{"union representatives met to select a chair", "the union selected a new leader"},
		},
		"sqli.boolean": {
			恶意: []string{"' OR 1=1--", "admin' AND 1=1#", "1 or 1 = 1"},
			正常: []string{"version 2 and build 3", "member 12 or 13"},
		},
		"sqli.subquery": {
			恶意: []string{"1 AND (SELECT COUNT(*) FROM users)>0", "id=(select password from users limit 1)"},
			正常: []string{"the query planner picks an index", "read the select from the menu"},
		},
		"sqli.error-based": {
			恶意: []string{"1 AND EXTRACTVALUE(1,CONCAT(0x7e,version()))", "1 AND UPDATEXML(1,1,1)"},
			正常: []string{"extract values from the report", "update the xml template"},
		},
		"sqli.time": {
			恶意: []string{"1' AND SLEEP(5)--", "id=1;SELECT BENCHMARK(1000000,MD5('x'))", "1 AND pg_sleep(3)"},
			正常: []string{"healthy sleep habits", "benchmark results for 2026"},
		},
		"sqli.stacked": {
			恶意: []string{"1; DROP TABLE users", "x'; delete from accounts;--"},
			正常: []string{"drop the old table", "insert new rows"},
		},
		"sqli.comment": {
			恶意: []string{"admin'--", "1 OR 1=1#", "x' /*"},
			正常: []string{"issue #42", "version 1.0-beta", "user@example.com"},
		},
		"sqli.schema": {
			恶意: []string{"UNION SELECT table_name FROM information_schema.tables", "select * from pg_catalog.pg_tables"},
			正常: []string{"the information schema is documented", "schema migrations run at boot"},
		},
		"xss.script": {
			恶意: []string{"<script>alert(1)</script>", "<SCRIPT src=//evil.test/x.js>"},
			正常: []string{"the script tag is documented", "JavaScript templates"},
		},
		"xss.event": {
			恶意: []string{"<img src=x onerror=alert(1)>", `" onmouseover="alert(document.cookie)`},
			正常: []string{"click handler documentation", "event load balancing"},
		},
		"xss.javascript-uri": {
			恶意: []string{"javascript:alert(document.cookie)", "JaVaScRiPt:alert(1)"},
			正常: []string{"ECMAScript syntax guide", "javascript frameworks"},
		},
		"xss.legacy-tag": {
			恶意: []string{"<marquee onstart=alert(1)>", "<vml:oval>", "<object data=x>"},
			正常: []string{"object storage documentation", "marquee lights at the venue"},
		},
		"xss.iframe": {
			恶意: []string{"<iframe src=//evil.test></iframe>"},
			正常: []string{"iframe embedding policy", "frame-ancestors directive"},
		},
		"xss.svg-event": {
			恶意: []string{"<svg onload=alert(1)>", `<svg><animate onbegin="alert(1)">`},
			正常: []string{`<svg width="10" height="10">`, "svg icon assets"},
		},
		"traversal.dotdot": {
			恶意: []string{"../../etc/passwd", `..\..\windows\win.ini`, "/static/../secret"},
			正常: []string{"/static/app.js", "no dots here", "file.name.txt"},
		},
		"traversal.encoded": {
			恶意: []string{"%2e%2e%2fetc/passwd", "%252e%252e%252fsecret"},
			正常: []string{"file%2Fname", "value%20with%20space"},
		},
		"traversal.sensitive": {
			恶意: []string{"/etc/passwd", `C:\boot.ini`},
			正常: []string{"/etc/hosts", "password policy"},
		},
		"traversal.null": {
			恶意: []string{"/etc/passwd%00.jpg", "file.php\x00.txt"},
			正常: []string{"null", "zero"},
		},
		"rce.shell-meta": {
			恶意: []string{"127.0.0.1; cat /etc/passwd", "x|whoami", "$(id)", "`curl http://evil.test`"},
			正常: []string{"I like cats", "a | b comparison", "bashful"},
		},
		"rce.php": {
			恶意: []string{"<?php system($_GET['c']); ?>", "phpinfo()"},
			正常: []string{"PHP is a language", "php_info documentation"},
		},
		"rce.java": {
			恶意: []string{"java.lang.Runtime.getRuntime().exec('id')", "Class.forName('Evil')"},
			正常: []string{"java.lang.String", "process builder pattern"},
		},
		"rce.shell-name": {
			恶意: []string{"cmd.exe /c whoami", "/bin/bash -c id", "powershell -enc AAA"},
			正常: []string{"power shell scripting", "bashful"},
		},
		"scanner.user-agent": {
			恶意: []string{"sqlmap/1.8.7#stable", "Mozilla/5.0 (compatible; Nuclei)"},
			正常: []string{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36", "curl/8.5.0"},
		},
		"scanner.path": {
			恶意: []string{"/.git/config", "/wp-admin/admin-ajax.php", "/phpmyadmin/index.php"},
			正常: []string{"/docs/setup", "/assets/app.js"},
		},
		"scanner.secret": {
			恶意: []string{"/.aws/credentials", "/server-status", "/actuator/env"},
			正常: []string{"/api/status", "/actuator/health"},
		},
	}
}

func Test默认规则样本覆盖全部规则(t *testing.T) {
	样本 := 默认规则样本()
	规则列表 := U默认规则()
	if len(规则列表) != len(样本) {
		t.Fatalf("rules=%d samples=%d", len(规则列表), len(样本))
	}
	for _, 规则 := range 规则列表 {
		样本项, 存在 := 样本[规则.UID]
		if !存在 {
			t.Fatalf("rule %s has no samples", 规则.UID)
		}
		if len(样本项.恶意) == 0 || len(样本项.正常) == 0 {
			t.Fatalf("rule %s needs at least one malicious and one benign sample", 规则.UID)
		}
	}
}
