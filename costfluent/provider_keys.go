package costfluent

// ProviderKey names what a provider connection connects to, spelled as the Public API sends and
// accepts it. The set grows with new connectors, so a client should tolerate a key it does not
// list. TestProviderKeysMatchTheContract holds this list to the API's declared values.
type ProviderKey string

const (
	ProviderKeyAws             ProviderKey = "aws"
	ProviderKeyAzure           ProviderKey = "azure"
	ProviderKeyGcp             ProviderKey = "gcp"
	ProviderKeyOracle          ProviderKey = "oracle"
	ProviderKeyAlibaba         ProviderKey = "alibaba"
	ProviderKeyIbm             ProviderKey = "ibm"
	ProviderKeyDigitalOcean    ProviderKey = "digitalOcean"
	ProviderKeyLinode          ProviderKey = "linode"
	ProviderKeyVultr           ProviderKey = "vultr"
	ProviderKeyHetzner         ProviderKey = "hetzner"
	ProviderKeyOvh             ProviderKey = "ovh"
	ProviderKeyScaleway        ProviderKey = "scaleway"
	ProviderKeyKubernetes      ProviderKey = "kubernetes"
	ProviderKeyOpenShift       ProviderKey = "openShift"
	ProviderKeyRancher         ProviderKey = "rancher"
	ProviderKeyDatabricks      ProviderKey = "databricks"
	ProviderKeySnowflake       ProviderKey = "snowflake"
	ProviderKeyBigQuery        ProviderKey = "bigQuery"
	ProviderKeyRedshift        ProviderKey = "redshift"
	ProviderKeyFirebolt        ProviderKey = "firebolt"
	ProviderKeyStarburst       ProviderKey = "starburst"
	ProviderKeyMongoDbAtlas    ProviderKey = "mongoDbAtlas"
	ProviderKeyPlanetScale     ProviderKey = "planetScale"
	ProviderKeyCockroachDb     ProviderKey = "cockroachDb"
	ProviderKeyRedis           ProviderKey = "redis"
	ProviderKeySupabase        ProviderKey = "supabase"
	ProviderKeyNeon            ProviderKey = "neon"
	ProviderKeyFauna           ProviderKey = "fauna"
	ProviderKeyTimescale       ProviderKey = "timescale"
	ProviderKeyClickHouseCloud ProviderKey = "clickHouseCloud"
	ProviderKeyDatadog         ProviderKey = "datadog"
	ProviderKeyNewRelic        ProviderKey = "newRelic"
	ProviderKeySplunk          ProviderKey = "splunk"
	ProviderKeyElastic         ProviderKey = "elastic"
	ProviderKeyGrafana         ProviderKey = "grafana"
	ProviderKeyDynatrace       ProviderKey = "dynatrace"
	ProviderKeyHoneycomb       ProviderKey = "honeycomb"
	ProviderKeyConfluent       ProviderKey = "confluent"
	ProviderKeyKafka           ProviderKey = "kafka"
	ProviderKeyPulsar          ProviderKey = "pulsar"
	ProviderKeyCloudflare      ProviderKey = "cloudflare"
	ProviderKeyFastly          ProviderKey = "fastly"
	ProviderKeyAkamai          ProviderKey = "akamai"
	ProviderKeyCloudFront      ProviderKey = "cloudFront"
	ProviderKeyGitHub          ProviderKey = "gitHub"
	ProviderKeyGitLab          ProviderKey = "gitLab"
	ProviderKeyCircleCi        ProviderKey = "circleCi"
	ProviderKeyBuildkite       ProviderKey = "buildkite"
	ProviderKeyCodefresh       ProviderKey = "codefresh"
	ProviderKeyVercel          ProviderKey = "vercel"
	ProviderKeyNetlify         ProviderKey = "netlify"
	ProviderKeyRailway         ProviderKey = "railway"
	ProviderKeyRender          ProviderKey = "render"
	ProviderKeyFly             ProviderKey = "fly"
	ProviderKeyHeroku          ProviderKey = "heroku"
	ProviderKeyOpenAi          ProviderKey = "openAi"
	ProviderKeyAnthropic       ProviderKey = "anthropic"
	ProviderKeyHuggingFace     ProviderKey = "huggingFace"
	ProviderKeyCohere          ProviderKey = "cohere"
	ProviderKeyReplicate       ProviderKey = "replicate"
	ProviderKeyAzureOpenAi     ProviderKey = "azureOpenAi"
	ProviderKeyAwsBedrock      ProviderKey = "awsBedrock"
	ProviderKeyVertexAi        ProviderKey = "vertexAi"
	ProviderKeyTwilio          ProviderKey = "twilio"
	ProviderKeySendGrid        ProviderKey = "sendGrid"
	ProviderKeyMailgun         ProviderKey = "mailgun"
	ProviderKeySalesforce      ProviderKey = "salesforce"
	ProviderKeyAtlassian       ProviderKey = "atlassian"
	ProviderKeySlack           ProviderKey = "slack"
	ProviderKeyZoom            ProviderKey = "zoom"
	ProviderKeyCustom          ProviderKey = "custom"
	ProviderKeyFocusExport     ProviderKey = "focusExport"
)

// ProviderKeys returns every key the Public API declares, in the contract's order.
func ProviderKeys() []ProviderKey {
	return []ProviderKey{
		ProviderKeyAws,
		ProviderKeyAzure,
		ProviderKeyGcp,
		ProviderKeyOracle,
		ProviderKeyAlibaba,
		ProviderKeyIbm,
		ProviderKeyDigitalOcean,
		ProviderKeyLinode,
		ProviderKeyVultr,
		ProviderKeyHetzner,
		ProviderKeyOvh,
		ProviderKeyScaleway,
		ProviderKeyKubernetes,
		ProviderKeyOpenShift,
		ProviderKeyRancher,
		ProviderKeyDatabricks,
		ProviderKeySnowflake,
		ProviderKeyBigQuery,
		ProviderKeyRedshift,
		ProviderKeyFirebolt,
		ProviderKeyStarburst,
		ProviderKeyMongoDbAtlas,
		ProviderKeyPlanetScale,
		ProviderKeyCockroachDb,
		ProviderKeyRedis,
		ProviderKeySupabase,
		ProviderKeyNeon,
		ProviderKeyFauna,
		ProviderKeyTimescale,
		ProviderKeyClickHouseCloud,
		ProviderKeyDatadog,
		ProviderKeyNewRelic,
		ProviderKeySplunk,
		ProviderKeyElastic,
		ProviderKeyGrafana,
		ProviderKeyDynatrace,
		ProviderKeyHoneycomb,
		ProviderKeyConfluent,
		ProviderKeyKafka,
		ProviderKeyPulsar,
		ProviderKeyCloudflare,
		ProviderKeyFastly,
		ProviderKeyAkamai,
		ProviderKeyCloudFront,
		ProviderKeyGitHub,
		ProviderKeyGitLab,
		ProviderKeyCircleCi,
		ProviderKeyBuildkite,
		ProviderKeyCodefresh,
		ProviderKeyVercel,
		ProviderKeyNetlify,
		ProviderKeyRailway,
		ProviderKeyRender,
		ProviderKeyFly,
		ProviderKeyHeroku,
		ProviderKeyOpenAi,
		ProviderKeyAnthropic,
		ProviderKeyHuggingFace,
		ProviderKeyCohere,
		ProviderKeyReplicate,
		ProviderKeyAzureOpenAi,
		ProviderKeyAwsBedrock,
		ProviderKeyVertexAi,
		ProviderKeyTwilio,
		ProviderKeySendGrid,
		ProviderKeyMailgun,
		ProviderKeySalesforce,
		ProviderKeyAtlassian,
		ProviderKeySlack,
		ProviderKeyZoom,
		ProviderKeyCustom,
		ProviderKeyFocusExport,
	}
}
