package port

// services maps common TCP ports to their standard IANA service names.
// Note: This catalog is strictly informative. An open TCP port indicates
// that a socket accepted the connection, but does not guarantee the identity
// or protocol compliance of the service running on that port.
var services = map[uint16]string{
	20:    "FTP-Data",
	21:    "FTP",
	22:    "SSH",
	23:    "Telnet",
	25:    "SMTP",
	53:    "DNS",
	67:    "DHCP-Server",
	68:    "DHCP-Client",
	69:    "TFTP",
	80:    "HTTP",
	88:    "Kerberos",
	110:   "POP3",
	119:   "NNTP",
	123:   "NTP",
	135:   "MS-RPC",
	137:   "NetBIOS-NS",
	138:   "NetBIOS-DGM",
	139:   "NetBIOS-SSN",
	143:   "IMAP",
	161:   "SNMP",
	162:   "SNMP-Trap",
	179:   "BGP",
	389:   "LDAP",
	443:   "HTTPS",
	445:   "SMB",
	465:   "SMTPS",
	514:   "Syslog",
	515:   "LPD",
	587:   "Submission (SMTP)",
	636:   "LDAPS",
	873:   "Rsync",
	993:   "IMAPS",
	995:   "POP3S",
	1080:  "SOCKS",
	1194:  "OpenVPN",
	1433:  "MS-SQL",
	1521:  "Oracle",
	1723:  "PPTP",
	1883:  "MQTT",
	2049:  "NFS",
	2181:  "ZooKeeper",
	3306:  "MySQL",
	3389:  "RDP",
	5060:  "SIP",
	5432:  "PostgreSQL",
	5672:  "RabbitMQ",
	5900:  "VNC",
	6379:  "Redis",
	8000:  "HTTP-Alt",
	8080:  "HTTP-Proxy",
	8443:  "HTTPS-Alt",
	8888:  "HTTP-Alt",
	9092:  "Kafka",
	9200:  "Elasticsearch",
	27017: "MongoDB",
}

// LookupService returns the known service name for a given port, or an empty string if unlisted.
func LookupService(port uint16) string {
	return services[port]
}
