package 引擎

// U默认规则 返回内置规则副本。规则刻意保守，宁可漏掉变形攻击，
// 也不把正常业务参数当成攻击。
func U默认规则() []U规则 {
	return []U规则{
		{UID: "sqli.union-select", U名称: "SQL union select", U类别: "sqli", U正则: `(?i)\bunion\s+(all\s+)?select\b`, U启用: true, UScore: 50},
		{UID: "sqli.boolean", U名称: "SQL boolean tautology", U类别: "sqli", U正则: `(?i)(\bor\b|\band\b)\s+['"]?\d+['"]?\s*=\s*['"]?\d+`, U启用: true, UScore: 50},
		{UID: "sqli.time", U名称: "SQL time delay", U类别: "sqli", U正则: `(?i)\b(sleep|benchmark|pg_sleep)\s*\(\s*\d+`, U启用: true, UScore: 50},
		{UID: "sqli.stacked", U名称: "SQL stacked statement", U类别: "sqli", U正则: `(?i);\s*(drop|alter|insert|update|delete)\s+`, U启用: true, UScore: 50},
		{UID: "sqli.comment", U名称: "SQL comment terminator", U类别: "sqli", U正则: `(?i)(--|#|/\*)\s*$`, U启用: true, UScore: 40},
		{UID: "sqli.schema", U名称: "SQL system catalog", U类别: "sqli", U正则: `(?i)\b(information_schema|sysobjects|pg_catalog)\b`, U启用: true, UScore: 40},

		{UID: "xss.script", U名称: "Script tag", U类别: "xss", U正则: `(?i)<\s*script\b`, U启用: true, UScore: 50},
		{UID: "xss.event", U名称: "Inline event handler", U类别: "xss", U正则: `(?i)\bon(click|error|load|mouseover|focus|submit|toggle)\s*=`, U启用: true, UScore: 40},
		{UID: "xss.javascript-uri", U名称: "JavaScript URI", U类别: "xss", U正则: `(?i)javascript\s*:`, U启用: true, UScore: 40},
		{UID: "xss.iframe", U名称: "Iframe injection", U类别: "xss", U正则: `(?i)<\s*iframe\b`, U启用: true, UScore: 40},
		{UID: "xss.svg-event", U名称: "SVG event handler", U类别: "xss", U正则: `(?i)<\s*svg\b[\s\S]{0,80}\bon[a-z]{3,15}\s*=`, U启用: true, UScore: 40},

		{UID: "traversal.dotdot", U名称: "Path traversal", U类别: "traversal", U正则: `(?i)(^|[/\\])\.\.([/\\]|$)`, U启用: true, UScore: 50},
		{UID: "traversal.encoded", U名称: "Encoded path traversal", U类别: "traversal", U正则: `(?i)(%2e%2e|%252e%252e)(/|%2f|\\|%5c|%252f|%255c)`, U启用: true, UScore: 50},
		{UID: "traversal.sensitive", U名称: "Sensitive file path", U类别: "traversal", U正则: `(?i)(/etc/passwd|/etc/shadow|boot\.ini|win\.ini)`, U启用: true, UScore: 50},
		{UID: "traversal.null", U名称: "Null byte injection", U类别: "traversal", U正则: `(?i)(%00|\x00)`, U启用: true, UScore: 40},

		{UID: "rce.shell-meta", U名称: "Shell metacharacter command", U类别: "rce", U正则: "(?i)(;|\\||&&|\\$\\(|`)\\s*(cat|wget|curl|bash|sh|nc|python|perl|id|whoami)\\b", U启用: true, UScore: 60},
		{UID: "rce.php", U名称: "PHP code marker", U类别: "rce", U正则: `(?i)<\?php\b|\bphpinfo\s*\(`, U启用: true, UScore: 50},
		{UID: "rce.java", U名称: "Java runtime access", U类别: "rce", U正则: `(?i)(java\.lang\.runtime|processbuilder|class\.forname)`, U启用: true, UScore: 50},
		{UID: "rce.shell-name", U名称: "Shell binary invocation", U类别: "rce", U正则: `(?i)(\bcmd\.exe\b|/bin/(ba)?sh\b|\bpowershell(\.exe)?\b)`, U启用: true, UScore: 50},

		{UID: "scanner.user-agent", U名称: "Scanner user agent", U类别: "scanner", U正则: `(?i)(sqlmap|nikto|nmap|masscan|acunetix|nessus|burpsuite|zgrab|nuclei)`, U启用: true, UScore: 30},
		{UID: "scanner.path", U名称: "Scanner path probe", U类别: "scanner", U正则: `(?i)(/\.git/|/\.svn/|/\.env\b|/wp-admin/|/phpmyadmin/)`, U启用: true, UScore: 30},
		{UID: "scanner.secret", U名称: "Secret file probe", U类别: "scanner", U正则: `(?i)(/\.aws/credentials|/\.ssh/id_rsa|/server-status|/actuator/env)`, U启用: true, UScore: 30},
	}
}
