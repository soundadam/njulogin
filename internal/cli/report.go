package cli

import (
	"fmt"
	"io"

	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/redact"
)

// Every record is one key=value line; portal fields pass through redact first.

func writeStatus(w io.Writer, status portal.Status) {
	if !status.Authenticated {
		fmt.Fprintln(w, "state=not_authenticated")
		return
	}
	fmt.Fprintln(w, "state=authenticated")
	fmt.Fprintf(w, "sessions=%d\n", len(status.Sessions))
	for index, session := range status.Sessions {
		fmt.Fprintf(w,
			"session_%d=%s %s start=%s service=%s mac=%s ip=redacted\n",
			index+1,
			redact.Account(session.Username),
			redact.Text(session.Fullname),
			redact.Unix(session.AcctStartTime),
			redact.Field(session.ServiceName, "unknown"),
			redact.MAC(session.MAC),
		)
	}
}

func writeOnlineRows(w io.Writer, rows []portal.Row) {
	fmt.Fprintf(w, "online_devices=%d\n", len(rows))
	for index, row := range rows {
		fmt.Fprintf(w,
			"online_%d=start=%s mac=%s ipv4=redacted ipv6=%s\n",
			index+1,
			redact.AnyUnix(row["acctstarttime"]),
			redact.MAC(row.Field("mac")),
			redact.Presence(row["user_ipv6"]),
		)
	}
}

func writeQuickRows(w io.Writer, rows []portal.Row) {
	fmt.Fprintf(w, "quick_login_devices=%d\n", len(rows))
	for index, row := range rows {
		fmt.Fprintf(w,
			"quick_%d=device_id=%v mac=%s name=%v\n",
			index+1,
			redact.Field(row.Field("id"), "unknown"),
			redact.MAC(row.Field("mac")),
			redact.Field(row.Field("device"), "unknown"),
		)
	}
}
