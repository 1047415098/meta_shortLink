package app

import (
	"whatsapp-analytics/internal/bootstrap"
	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/modules/analytics"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/tracking"
)

type App = bootstrap.App
type Config = config.Config
type Summary = analytics.Summary
type Trend = analytics.Trend

var New = bootstrap.New
var LoadConfig = config.LoadConfig
var mustLocation = config.Location
var validTarget = links.ValidTarget
var classify = tracking.Classify
var csvSafe = analytics.CSVSafe

type Ad = analytics.Ad
