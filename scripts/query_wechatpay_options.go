package main

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "one-api.db"
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		dbPath = os.Args[1]
	}

	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		// keep it simple; busy timeout not critical for read-only inspection
		dsn = dsn + "?_busy_timeout=30000"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Printf("open db failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT key, value FROM options WHERE key LIKE 'WeChatPay%'`)
	if err != nil {
		fmt.Printf("query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			fmt.Printf("scan failed: %v\n", err)
			os.Exit(1)
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("rows err: %v\n", err)
		os.Exit(1)
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("DB=%s\n", dbPath)
	if len(keys) == 0 {
		fmt.Println("No WeChatPay* options found.")
		return
	}
	for _, k := range keys {
		v := m[k]
		// avoid printing full secrets
		if k == "WeChatPayAPIv3Key" || k == "WeChatPayPrivateKey" {
			if v == "" {
				fmt.Printf("%s = <EMPTY>\n", k)
			} else {
				fmt.Printf("%s = <SET len=%d>\n", k, len(v))
			}
			continue
		}
		fmt.Printf("%s = %q\n", k, v)
	}
}

