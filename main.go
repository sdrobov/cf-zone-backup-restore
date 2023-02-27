package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

type (
	zoneOwner struct {
		Id    string `json:"id"`
		Email string `json:"email"`
		Type  string `json:"type"`
	}
	zoneAccount struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	zonePlan struct {
		Id           string `json:"id"`
		Name         string `json:"name"`
		Price        int    `json:"price"`
		Currency     string `json:"currency"`
		Frequency    string `json:"frequency"`
		LegacyId     string `json:"legacy_id"`
		IsSubscribed bool   `json:"is_subscribed"`
		CanSubscribe bool   `json:"can_subscribe"`
	}
	zonePlanPending struct {
		Id           string `json:"id"`
		Name         string `json:"name"`
		Price        int    `json:"price"`
		Currency     string `json:"currency"`
		Frequency    string `json:"frequency"`
		LegacyId     string `json:"legacy_id"`
		IsSubscribed bool   `json:"is_subscribed"`
		CanSubscribe bool   `json:"can_subscribe"`
	}
	zone struct {
		Id                  string           `json:"id"`
		Name                string           `json:"name"`
		DevelopmentMode     int              `json:"development_mode"`
		OriginalNameServers []string         `json:"original_name_servers"`
		OriginalRegistrar   string           `json:"original_registrar"`
		OriginalDnshost     string           `json:"original_dnshost"`
		Owner               *zoneOwner       `json:"owner"`
		Account             *zoneAccount     `json:"account"`
		Permissions         []string         `json:"permissions"`
		Plan                *zonePlan        `json:"plan"`
		PlanPending         *zonePlanPending `json:"plan_pending"`
		Status              string           `json:"status"`
		Paused              bool             `json:"paused"`
		Type                string           `json:"type"`
		NameServers         []string         `json:"name_servers"`
	}
	listZonesResponse struct {
		Success  bool          `json:"success"`
		Errors   []interface{} `json:"errors"`
		Messages []interface{} `json:"messages"`
		Result   []*zone       `json:"result"`
	}
	zoneDnsRecordMeta struct {
		AutoAdded bool   `json:"auto_added"`
		Source    string `json:"source"`
	}
	zoneDnsRecord struct {
		Id        string             `json:"id"`
		Type      string             `json:"type"`
		Name      string             `json:"name"`
		Content   string             `json:"content"`
		Proxiable bool               `json:"proxiable"`
		Proxied   bool               `json:"proxied"`
		Comment   string             `json:"comment"`
		Tags      []string           `json:"tags"`
		Ttl       int                `json:"ttl"`
		Locked    bool               `json:"locked"`
		ZoneId    string             `json:"zone_id"`
		ZoneName  string             `json:"zone_name"`
		Meta      *zoneDnsRecordMeta `json:"meta"`
	}
	listZoneDnsRecordsResponse struct {
		Success  bool             `json:"success"`
		Errors   []interface{}    `json:"errors"`
		Messages []interface{}    `json:"messages"`
		Result   []*zoneDnsRecord `json:"result"`
	}
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "email",
				Aliases:  []string{"e"},
				Usage:    "cf account email address",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "key",
				Aliases:  []string{"k"},
				Usage:    "cf account api key",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "token",
				Aliases:  []string{"t"},
				Usage:    "cf account api token",
				Required: false,
			},
			&cli.StringFlag{
				Name:        "dir",
				Aliases:     []string{"d"},
				Usage:       "backup directory",
				Required:    false,
				DefaultText: "./",
			},
			&cli.StringFlag{
				Name:        "url",
				Aliases:     []string{"u"},
				Usage:       "cf api url",
				Required:    false,
				DefaultText: "https://api.cloudflare.com/client/v4/",
			},
			&cli.BoolFlag{
				Name:        "verbose",
				Aliases:     []string{"v"},
				Usage:       "verbose output",
				Required:    false,
				DefaultText: "false",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "backup",
				Usage:  "backup zones",
				Action: backupZones,
			},
			{
				Name:   "restore",
				Usage:  "restore zones",
				Action: restoreZones,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(fmt.Errorf("error running app: %w", err))
	}
}

func restoreZones(ctx *cli.Context) error {
	email := ctx.String("email")
	key := ctx.String("key")
	token := ctx.String("token")
	if email == "" && key == "" && token == "" {
		return fmt.Errorf("email & key or token must be provided")
	}
	dir := ctx.String("dir")
	url := ctx.String("url")
	if url == "" {
		url = "https://api.cloudflare.com/client/v4/"
	}
	verbose := ctx.Bool("verbose")

	backupFile := filepath.Join(dir, "backup.json")
	backup, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("error opening backup file: %w", err)
	}
	defer func(backup *os.File) {
		err := backup.Close()
		if err != nil {
			log.Printf("error closing backup file: %v", err)
		}
	}(backup)

	decoder := json.NewDecoder(backup)
	var dnsRecords []*zoneDnsRecord
	err = decoder.Decode(&dnsRecords)
	if err != nil {
		return fmt.Errorf("error decoding backup file: %w", err)
	}

	if verbose {
		fmt.Printf("restoring %d dns records\n", len(dnsRecords))
	}

	existingDnsRecords, err := getAllDnsRecords(ctx)
	if err != nil {
		return fmt.Errorf("error getting existing dns records: %w", err)
	}

	var forUpdate, forDelete, forCreate []*zoneDnsRecord
	existingDnsRecordsMap := make(map[string]*zoneDnsRecord, len(existingDnsRecords))
	for _, dnsRecord := range existingDnsRecords {
		existingDnsRecordsMap[dnsRecord.Id] = dnsRecord
	}
	backupDnsRecordsMap := make(map[string]*zoneDnsRecord, len(dnsRecords))
	for _, dnsRecord := range dnsRecords {
		existingDnsRecord, ok := existingDnsRecordsMap[dnsRecord.Id]
		if !ok {
			forCreate = append(forCreate, dnsRecord)
			continue
		}
		if !reflect.DeepEqual(dnsRecord, existingDnsRecord) {
			forUpdate = append(forUpdate, dnsRecord)
		}
	}
	for _, existingDnsRecord := range existingDnsRecords {
		_, ok := backupDnsRecordsMap[existingDnsRecord.Id]
		if !ok {
			forDelete = append(forDelete, existingDnsRecord)
		}
	}

	total := len(forCreate) + len(forUpdate) + len(forDelete)
	i := 0
	for _, dnsRecord := range forUpdate {
		i += 1
		if verbose {
			fmt.Printf("updating dns record %s; %d/%d\n", dnsRecord.Name, i, total)
		}
		body := &bytes.Buffer{}
		err := json.NewEncoder(body).Encode(dnsRecord)
		if err != nil {
			return fmt.Errorf("error encoding dns record: %w", err)
		}
		client := &http.Client{}
		req, err := http.NewRequest("PUT", url+"zones/"+dnsRecord.ZoneId+"/dns_records/"+dnsRecord.Id, body)
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}

		if email != "" && key != "" {
			req.Header.Add("X-Auth-Email", email)
			req.Header.Add("X-Auth-Key", key)
		} else {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %w", err)
		}
		err = resp.Body.Close()
		if err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}
	for _, dnsRecord := range forDelete {
		i += 1
		if verbose {
			fmt.Printf("deleting dns record %s; %d/%d\n", dnsRecord.Name, i, total)
		}
		client := &http.Client{}
		req, err := http.NewRequest("DELETE", url+"zones/"+dnsRecord.ZoneId+"/dns_records/"+dnsRecord.Id, nil)
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}

		if email != "" && key != "" {
			req.Header.Add("X-Auth-Email", email)
			req.Header.Add("X-Auth-Key", key)
		} else {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %w", err)
		}
		err = resp.Body.Close()
		if err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}
	for _, dnsRecord := range forCreate {
		i += 1
		if verbose {
			fmt.Printf("creating dns record %s; %d/%d\n", dnsRecord.Name, i, total)
		}
		body := &bytes.Buffer{}
		err := json.NewEncoder(body).Encode(dnsRecord)
		if err != nil {
			return fmt.Errorf("error encoding dns record: %w", err)
		}
		client := &http.Client{}
		req, err := http.NewRequest("POST", url+"zones/"+dnsRecord.ZoneId+"/dns_records", body)
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}

		if email != "" && key != "" {
			req.Header.Add("X-Auth-Email", email)
			req.Header.Add("X-Auth-Key", key)
		} else {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %w", err)
		}
		err = resp.Body.Close()
		if err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}

	return nil
}

func backupZones(ctx *cli.Context) error {
	dir := ctx.String("dir")
	forBackup, err := getAllDnsRecords(ctx)
	if err != nil {
		return fmt.Errorf("error getting all dns records: %w", err)
	}

	backupFile, err := os.Create(dir + "backup.json")
	if err != nil {
		return fmt.Errorf("error creating backup file: %w", err)
	}
	defer func(backupFile *os.File) {
		err := backupFile.Close()
		if err != nil {
			log.Printf("error closing backup file: %v", err)
		}
	}(backupFile)

	encoder := json.NewEncoder(backupFile)
	err = encoder.Encode(forBackup)
	if err != nil {
		return fmt.Errorf("error encoding backup file: %w", err)
	}

	return nil
}

func getAllDnsRecords(ctx *cli.Context) ([]*zoneDnsRecord, error) {
	email := ctx.String("email")
	key := ctx.String("key")
	token := ctx.String("token")
	if email == "" && key == "" && token == "" {
		return nil, fmt.Errorf("email & key or token must be provided")
	}
	url := ctx.String("url")
	if url == "" {
		url = "https://api.cloudflare.com/client/v4/"
	}
	verbose := ctx.Bool("verbose")

	zones, err := getAllZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting all zones: %w", err)
	}

	zoneDnsRecords := make([]*zoneDnsRecord, 0, len(zones))
	client := &http.Client{}
	for i, zone := range zones {
		if verbose {
			fmt.Printf("Fetching zone %s; %d/%d\n", zone.Name, i+1, len(zones))
		}
		page := 1
		for {
			req, err := http.NewRequest("GET", url+"zones/"+zone.Id+"/dns_records?page="+strconv.Itoa(page), nil)
			if err != nil {
				return nil, fmt.Errorf("error creating request: %w", err)
			}

			if email != "" && key != "" {
				req.Header.Add("X-Auth-Email", email)
				req.Header.Add("X-Auth-Key", key)
			} else {
				req.Header.Add("Authorization", "Bearer "+token)
			}
			req.Header.Add("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("error sending request: %w", err)
			}

			decoder := json.NewDecoder(resp.Body)
			var response listZoneDnsRecordsResponse
			err = decoder.Decode(&response)
			if err != nil {
				return nil, fmt.Errorf("error decoding response: %w", err)
			}

			if !response.Success {
				return nil, fmt.Errorf("error response: %v", response.Errors)
			}

			if len(response.Result) == 0 {
				break
			}
			page++
			zoneDnsRecords = append(zoneDnsRecords, response.Result...)
			err = resp.Body.Close()
			if err != nil {
				log.Printf("error closing response body: %v", err)
			}
		}
	}

	return zoneDnsRecords, nil
}

func getAllZones(ctx *cli.Context) ([]*zone, error) {
	var zones []*zone

	email := ctx.String("email")
	key := ctx.String("key")
	token := ctx.String("token")
	if email == "" && key == "" && token == "" {
		return zones, fmt.Errorf("email & key or token must be provided")
	}
	url := ctx.String("url")
	if url == "" {
		url = "https://api.cloudflare.com/client/v4/"
	}
	verbose := ctx.Bool("verbose")

	client := &http.Client{}
	page := 1
	for {
		req, err := http.NewRequest("GET", url+"zones?page="+strconv.Itoa(page), nil)
		if err != nil {
			return zones, fmt.Errorf("error creating request: %w", err)
		}

		if email != "" && key != "" {
			req.Header.Add("X-Auth-Email", email)
			req.Header.Add("X-Auth-Key", key)
		} else {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return zones, fmt.Errorf("error sending request: %w", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Printf("error closing response body: %v", err)
			}
		}(resp.Body)

		decoder := json.NewDecoder(resp.Body)
		var response listZonesResponse
		err = decoder.Decode(&response)
		if err != nil {
			return zones, fmt.Errorf("error decoding response: %w", err)
		}

		if !response.Success {
			return zones, fmt.Errorf("error response: %v", response.Errors)
		}

		if len(response.Result) == 0 {
			break
		}

		if verbose {
			fmt.Printf("Found %d zones\n", len(response.Result))
		}

		zones = append(zones, response.Result...)
		page++
	}

	return zones, nil
}
