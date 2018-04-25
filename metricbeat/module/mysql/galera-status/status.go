/*
Package galera-status fetches MySQL Galera server status metrics.

For more information on the query it uses, see:
http://dev.mysql.com/doc/refman/5.7/en/show-status.html
http://galeracluster.com/documentation-webpages/galerastatusvariables.html
*/
package galerastatus

import (
	"database/sql"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/elastic/beats/metricbeat/module/mysql"

	"github.com/pkg/errors"
)

var (
	debugf = logp.MakeDebug("mysql-galera-status")
)

// init registers the MetricSet with the central registry.
func init() {
	mb.Registry.MustAddMetricSet("mysql", "galera-status", New, 
		mb.WithHostParser(mysql.ParseDSN),
	)
}

// MetricSet for fetching Galera-MySQL server status
type MetricSet struct {
	mb.BaseMetricSet
	db *sql.DB
	queryMode string
}

// New create a new instance of the MetricSet
// Loads query_mode config setting from the config file
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	config := struct {
		QueryMode string `config:"galera.query_mode"`
	}{
		QueryMode: "small",
	}

	err := base.Module().UnpackConfig(&config)
	if err != nil {
		return nil, err
	}

	logp.Debug("cfgfile", "Using %s metricset for fetching data.", config.QueryMode)

	return &MetricSet{
		BaseMetricSet:	base,
		queryMode:		config.QueryMode,
		}, nil
}

// Fetch methods implements the data gathering and data conversion to the right format
// It returns the event which is then forward to the output.
func (m *MetricSet) Fetch() (common.MapStr, error) {
	if m.db == nil {
		var err error
		m.db, err = mysql.NewDB(m.HostData().URI)
		if err != nil {
			return nil, errors.Wrap(err, "Galera-status fetch failed")
		}
	}

	status, err := m.loadStatus(m.db)
	if err != nil {
		return nil, err
	}

	event, err := eventMapping(status, m.queryMode)
	if err != nil {
		return nil, err
	}

	if m.Module().Config().Raw {
		event["raw"], err = rawEventMapping(status, m.queryMode)

		if err != nil {
			return nil, err
		}
	}
	return event, nil
}

// loadStatus loads all status entries from the given database into an array.
func (m *MetricSet) loadStatus(db *sql.DB) (map[string]string, error) {
	// Returns the global status, also for versions previous 5.0.2
	rows, err := db.Query("SHOW /*!50002 GLOBAL */ STATUS LIKE 'wsrep_%';")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	galeraStatus := map[string]string{}

	for rows.Next() {
		var name string
		var value string

		err = rows.Scan(&name, &value)
		if err != nil {
			return nil, err
		}

		galeraStatus[name] = value
	}

	return galeraStatus, nil
}
