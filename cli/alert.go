package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/api"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/prometheus/alertmanager/cli/format"
	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/alertmanager/pkg/parse"
)

type alertQueryCmd struct {
	inhibited, silenced bool
	matcherGroups       []string
}

const alertHelp = `View and search through current alerts.

Amtool has a simplified prometheus query syntax, but contains robust support for
bash variable expansions. The non-option section of arguments constructs a list
of "Matcher Groups" that will be used to filter your query. The following
examples will attempt to show this behaviour in action:

amtool alert query alertname=foo node=bar

	This query will match all alerts with the alertname=foo and node=bar label
	value pairs set.

amtool alert query foo node=bar

	If alertname is omitted and the first argument does not contain a '=' or a
	'=~' then it will be assumed to be the value of the alertname pair.

amtool alert query 'alertname=~foo.*'

	As well as direct equality, regex matching is also supported. The '=~' syntax
	(similar to prometheus) is used to represent a regex match. Regex matching
	can be used in combination with a direct match.
`

func configureAlertCmd(app *kingpin.Application) {
	var (
		a        = &alertQueryCmd{}
		alertCmd = app.Command("alert", alertHelp).PreAction(requireAlertManagerURL)
		queryCmd = alertCmd.Command("query", alertHelp).Default()
	)
	queryCmd.Flag("inhibited", "Show inhibited alerts").Short('i').BoolVar(&a.inhibited)
	queryCmd.Flag("silenced", "Show silenced alerts").Short('s').BoolVar(&a.silenced)
	queryCmd.Arg("matcher-groups", "Query filter").StringsVar(&a.matcherGroups)
	queryCmd.Action(a.queryAlerts)
}

func (a *alertQueryCmd) queryAlerts(ctx *kingpin.ParseContext) error {
	var filterString = ""
	if len(a.matcherGroups) == 1 {
		// If the parser fails then we likely don't have a (=|=~|!=|!~) so lets
		// assume that the user wants alertname=<arg> and prepend `alertname=`
		// to the front.
		_, err := parse.Matcher(a.matcherGroups[0])
		if err != nil {
			filterString = fmt.Sprintf("{alertname=%s}", a.matcherGroups[0])
		} else {
			filterString = fmt.Sprintf("{%s}", strings.Join(a.matcherGroups, ","))
		}
	} else if len(a.matcherGroups) > 1 {
		filterString = fmt.Sprintf("{%s}", strings.Join(a.matcherGroups, ","))
	}

	c, err := api.NewClient(api.Config{Address: alertmanagerURL.String()})
	if err != nil {
		return err
	}
	alertAPI := client.NewAlertAPI(c)
	fetchedAlerts, err := alertAPI.List(context.Background(), filterString, a.silenced, a.inhibited)
	if err != nil {
		return err
	}

	formatter, found := format.Formatters[output]
	if !found {
		return errors.New("unknown output formatter")
	}
	return formatter.FormatAlerts(fetchedAlerts)
}
