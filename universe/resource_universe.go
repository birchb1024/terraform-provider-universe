package universe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resourceCustom() *schema.Resource {
	return &schema.Resource{
		Create: onCreate,
		Read:   onRead,
		Update: onUpdate,
		Delete: onDelete,
		Exists: onExists,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		SchemaVersion: 1,

		Schema: map[string]*schema.Schema{
			"executor": {
				Description: "The name of the program to run. e.g. python",
				Type:        schema.TypeString,
				Optional:    true,
			},

			"script": {
				Description: "The path to the script passed as the first argument to 'executor'.",
				Type:        schema.TypeString,
				Optional:    true,
			},

			"config": {
				Description:      "The information (in JSON format) managed by Terraform plan and apply.",
				Type:             schema.TypeString,
				Required:         true,
				ValidateFunc:     validation.StringIsNotWhiteSpace,
				DiffSuppressFunc: diffSuppressComputed,
			},

			"id_key": {
				Description:  "The name of the key which holds the unique identifier of the resource. e.g. 'id'",
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
		},
	}
}

// diffSuppressComputed - Only different if the non @ fields have changed.
// remove the @ fields from the two JSON strings and then compare them.
func diffSuppressComputed(_, old, new string, _ *schema.ResourceData) bool {

	removeComputed := func(jsonish string) string {
		var x interface{}
		jstr, err := decodeConfigToJSON([]byte(jsonish))
		err = json.Unmarshal(jstr, &x)
		if err != nil {
			log.Printf("diffSuppressComputed():func[removeComputed] Could not parse Interface: %#v ", err)
			return ""
		}
		xmap, ok := x.(map[string]interface{})
		if !ok {
			log.Printf("diffSuppressComputed():func[removeComputed] Could not parse Map: %#v ", ok)
			return ""
		}
		for attrName := range xmap {
			if strings.HasPrefix(attrName, "@") {
				delete(xmap, attrName)
			}
		}
		xbytes, err := json.Marshal(xmap)
		if err != nil {
			return ""
		}
		return string(xbytes[:])
	}

	newJSON := removeComputed(new)
	oldJSON := removeComputed(old)

	result := newJSON == oldJSON
	log.Printf("diffSuppressComputed() %#v for\n* %#v\n* %#v \n", result, old, new)
	log.Printf("diffSuppressComputed() Compared Structs:\n* %#v\n* %#v\n", oldJSON, newJSON)
	return result
}

func onCreate(d *schema.ResourceData, m interface{}) error {
	_, err := callExecutor("create", d, m)
	return err
}

func onRead(d *schema.ResourceData, m interface{}) error {
	_, err := callExecutor("read", d, m)
	return err
}

func onUpdate(d *schema.ResourceData, m interface{}) error {
	_, err := callExecutor("update", d, m)
	return err
}

func onDelete(d *schema.ResourceData, m interface{}) error {
	_, err := callExecutor("delete", d, m)
	return err
}

func onExists(d *schema.ResourceData, m interface{}) (bool, error) {
	return callExecutor("exists", d, m)
}

func getFromDefaultsOrResource(name string, defaults map[string]interface{}, d ResourceLike, required bool) (string, bool) {
	//
	log.Printf("getFromDefaultsOrResource() field %s in %#v or %#v\n", name, defaults, required)

	var result string
	found := false
	value, ok := defaults[name]
	if ok {
		if str, ok := value.(string); ok {
			result = str
			found = true
		}
	}
	dv, dok := d.GetOk(name)
	if dok {
		str, ok := dv.(string)
		if ok {
			result = str
			found = true
		}
	}
	return result, found
}

// callExecutor - function to handle all the CRUDE. Returns with bool for 'exit'  all other responses
// are made in updates of the schema.ResourceData.
func callExecutor(event string, d ResourceLike, providerConfig interface{}) (bool, error) {

	effectiveDefaults, id, err := extractEssentialFields(event, d, providerConfig)
	if err != nil {
		return false, err
	}
	log.Printf("effectiveDefaults = %#v", effectiveDefaults)

	log.Printf("Executing: %s", event)
	configData, err := getConfigFromTF(d)
	if err != nil {
		return false, err
	}
	log.Printf("Executing: %s", string(configData))

	pwd, _ := os.Getwd()
	scriptPath, err := filepath.Abs(pwd + "/" + effectiveDefaults["script"].(string))
	if err != nil {
		return false, err
	}

	cmd := exec.Command(effectiveDefaults["executor"].(string), scriptPath, event)
	cmd.Env = makeEnvironment(id, effectiveDefaults)

	if event == "delete" {
		cmd.Stdin = bytes.NewReader([]byte{})
	} else {
		cmd.Stdin = bytes.NewReader(configData)
	}
	// Call the executor
	rawResponse, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return false, fmt.Errorf("command error: %s", string(ee.Stderr))
		}
		return false, err
	}
	response, err := jsonSafeUnmarshal(rawResponse, err)
	if err != nil {
		return false, err
	}
	// Process the response
	if event == "exists" {
		var exists bool
		err = json.Unmarshal(rawResponse, &exists) // Need special unmarshall for atomic types
		if err != nil {
			return false, fmt.Errorf("expecting boolean from subprocess, got '%#v'", string(rawResponse))
		}
		return exists, nil
	} else if event == "delete" {
		d.SetId("")
	} else {
		responseMap, ok := response.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("expecting map[string]interface{} from subprocess, got '%#v'", string(rawResponse))
		}
		// Get the id_key field from the response and move it into the special id member in the resourceData
		idKey := effectiveDefaults["id_key"].(string)
		if event == "create" {
			idRaw, ok := responseMap[idKey]
			if !ok {
				return false, fmt.Errorf("missing id attribute '%s' in response: %s", idKey, string(rawResponse))
			}
			id, ok := idRaw.(string)
			if !ok {
				return false, fmt.Errorf("expected string in id attribute '%s' in response but got: %#v", idKey, idRaw)
			}
			d.SetId(id)
		}
		delete(responseMap, idKey)

		// Now set the payload in the resource data 'config' field
		payloadBytes, err := json.Marshal(responseMap)
		if err != nil {
			return false, err
		}
		log.Printf("Executed: setting data to: %s", string(payloadBytes))
	}

	return false, err
}

// jsonSafeUnmarshal - copes with empty input
func jsonSafeUnmarshal(result []byte, err error) (interface{}, error) {
	var resource interface{}
	if len(result) == 0 {
		resource = nil
	} else {
		err = json.Unmarshal(result, &resource)
		if err != nil {
			return nil, err
		}
	}
	return resource, err
}

// makeEnvironment - Add the id and the 'environment' to the parent process environment,
// returning []string
func makeEnvironment(id string, effectiveDefaults map[string]interface{}) []string {
	environ := os.Environ()
	environ = append(environ, fmt.Sprintf("%s=%s", effectiveDefaults["id_key"], id))
	for k, v := range effectiveDefaults {
		if s, ok := v.(string); ok {
			e := fmt.Sprintf("%s=%s", k, s)
			environ = append(environ, e)
			log.Printf("Executing: with env var from default: %s", e)
		}
		if k == "environment" {
			if env, ok := v.(map[string]interface{}); ok {
				for envname, enval := range env {
					e := fmt.Sprintf("%s=%s", envname, enval)
					environ = append(environ, e)
					log.Printf("Executing: with env var from environment': %s", e)
				}
			}
		}
	}
	return environ
}

// extractEssentialFields - get the important fields from the provider config or resourceData.
// returning the a map[string] of the fields and the id field
func extractEssentialFields(event string, d ResourceLike, providerConfig interface{}) (map[string]interface{}, string, error) {
	essentialFields := map[string]bool{
		// map[field name]mandatory?
		"environment": false,
		"executor":    true,
		"id_key":      true,
		"script":      true,
	}
	stringFields := []string{"id_key", "executor", "script"}

	var effectiveDefaults = map[string]interface{}{}

	id := d.Id()
	log.Printf("callExecutor() '%s' %s %#v", id, event, providerConfig)
	for n := range essentialFields {
		log.Printf("callExecutor() ResourceData field %s = %#v", n, d.Get(n))
	}
	// Validate provider configuration
	if providerConfig != nil {
		var ok bool
		effectiveDefaults, ok = providerConfig.(map[string]interface{})
		if !ok {
			return nil, "", fmt.Errorf("was expecting map[string]interface{} in provider configuration, got %#v", providerConfig)
		}
	}
	// Extract essential fields from provider configuration or resource data
	for k, required := range essentialFields {
		value, found := getFromDefaultsOrResource(k, effectiveDefaults, d, required)
		if (!found) && required {
			return effectiveDefaults, id, fmt.Errorf("missing required field %s in %v or %#v", k, effectiveDefaults, d)
		}
		if !found {
			continue
		}
		effectiveDefaults[k] = value
		log.Printf("getFromDefaultsOrResource => field %s = %#v", k, value)
	}
	// Ensure fields are string
	for _, stringFieldName := range stringFields {
		if f, ok := effectiveDefaults[stringFieldName]; ok {
			if _, ok := f.(string); !ok {
				return effectiveDefaults, id, fmt.Errorf("expected %s to be string, but got %#v", stringFieldName, f)
			}
		}
	}
	return effectiveDefaults, id, nil
}

// getConfigFromTF - Validate and extract the 'config' JSON field from the resourceData, returning []byte
func getConfigFromTF(d ResourceLike) ([]byte, error) {
	dr, ok := d.GetOk("config")
	if !ok || dr == nil {
		return nil, fmt.Errorf("missing 'config'")
	}
	js, ok := dr.(string)
	if !ok {
		return nil, fmt.Errorf("expected string in 'config', but got: %#v", dr)
	}
	db := []byte(js)

	return decodeConfigToJSON(db)
}

func decodeConfigToJSON(str []byte) ([]byte, error) {
	attributes := map[string]interface{}{}
	// Try extraction from JSON
	err := json.Unmarshal(str, &attributes)
	if err == nil {
		return json.Marshal(attributes)
	}
	// Try extraction from YAML
	err = yaml.Unmarshal(str, &attributes)
	if err == nil {
		return json.Marshal(attributes)
	}
	err = toml.Unmarshal(str, &attributes)
	// Try extraction from TOML
	if err == nil {
		log.Printf("decodeConfigToJSON:toml - passed")
		return json.Marshal(attributes)
	}
	log.Printf("decodeConfigToJSON - expected JSON/YAML/TOML in 'config' but got: %#v", str)
	return nil, fmt.Errorf("expected JSON/YAML/TOML in 'config' but got: %#v", str)
}
