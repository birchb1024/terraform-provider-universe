Terraform Provider Multiverse
==================

You can use this provider instead of writing your own Terraform Custom Provider in the Go language. Just write your 
logic in any language you prefer (python, node, java, shell) and use it with this provider. You can write a script that
will be used to create, update or destroy external resources that are not supported by Terraform providers. 

Maintainers
-----------

The MobFox DevOps team at [MobFox](https://www.mobfox.com/) maintains the original provider. 

This is the birchb1024 fork, maintained by Peter Birch. This fork is no longer compatible with the original.

Requirements
------------

-	[Terraform](https://www.terraform.io/downloads.html) 0.13
-	[Go](https://golang.org/doc/install) 1.15.2 (to build the provider plugin)

Building The Provider
---------------------

Clone repository to: `$GOPATH/src/github.com/birchb1024/terraform-provider-multiverse`

```sh
$ mkdir -p $GOPATH/src/github.com/mobfox; cd $GOPATH/src/github.com/mobfox
$ git clone git@github.com:birchb1024/terraform-provider-multiverse.git
```

Enter the provider directory and build the provider

```sh
$ cd $GOPATH/src/github.com/mobfox/terraform-provider-multiverse
$ make build
```

#Using the provider

Check the `examples/` directory

Here an example of a provider which creates a json file in /tmp and stores data in it. 
This is implemented in the json_file example directory.


## Example TF

Here's a TF which creates three JSON files in /tmp.

```hcl
terraform {
  required_version = ">= 0.13.0"
  required_providers {
    multiverse = {
      source = "github.com/mobfox/multiverse"
      version = ">=0.0.1"
    }
    linux = {
      source = "github.com/mobfox/linux"
      version = ">=0.0.1"
    }
  }
}
provider "multiverse" {
  executor = "python3"
  script = "json_file.py"
  id_key = "filename"
  environment = {
    api_token = "redacted"
    servername = "api.example.com"
    api_token = "redacted"
  }
  computed = jsonencode([
    "created"])
}

resource "multiverse_json_file" "h" {
  config = jsonencode({
    "name": "Don't Step On My Blue Suede Shoes",
    "created-by" : "Elvis Presley",
    "where" : "Gracelands"
    "hit" : "Gold"

  })
}

resource "multiverse_json_file" "hp" {
  config = jsonencode({
    "name": "Another strange resource",
    "main-character" : "Harry Potter",
    "nemesis" : "Tom Riddle",
    "likes" : [
      "Ginny Weasley",
      "Ron Weasley"
    ]
  })
}

resource "linux_json_file" "i" { // Requires a plugin copy of multiverse
  executor = "python3"
  script = "json_file.py"
  id_key = "filename"
  computed = jsonencode(["created"])
  config = jsonencode({
    "name": "Fake strange resource"
  })
}

output "hp_name" {
  value = jsondecode(multiverse_json_file.hp.config)["name"]
}

output "hp_created" {
  value = jsondecode(multiverse_json_file.hp.dynamic)["created"]
}
```

- When you run `terraform apply` the resource will be created / updated
- When you run `terraform destroy` the resource will be destroyed

#### Attributes

* `executor (string)` could be anything like python, bash, sh, node, java, awscli ... etc
* `script (string)` the path to your script or program to run, the script must exit with code 0 and return a valid json string
* `id_key (string)` the key of returned result to be used as id by terraform
* `config (JSON string)` must be a valid JSON string. This contains the configuration of the resource and is managed by Terraform.
* `computed (JSON string)` - a list of field names which are dymanic, ie computed by the executor and should be ignored by TF plan
* `dynamic (JSON string)` - a JSON string generated by the provider containing the fields from the executor which have been identified in the provider`computed` list 

#### Handling Dynamic Data from the Executor

The `config` field in the provider attributes is monitored by Terraform plan for changes because it is a Required field.
Any changes are detected and put into the plan. However, your provider may generated attributes dynamically (such as the creation
date) of a resource. When you list these dynamic fields in the `computed` field in the provider configuration or resource blocks, 
multiverse moves these fields into the `dynamic` field. The `dynamic` field is marked 'Computed' and is ignored by Terraform plan. As follows:

```hcl-terraform
resource "json_file" "h" {
  provider = multiverse // because Terraform does not scan local providers for resource types.
  executor = "python3"
  script = "json_file.py"
  id_key = "filename"
  computed = jsonencode(["created"])
  config = jsonencode({
      "name": "test-terraform-test-43",
      "created-by" : "Elvis Presley",
      "where" : "gracelands"
    })
}
```

After the plan is applied the tfstate file will then contain information:

```hcl-terraform
resource "json_file" "h" {
    computed = jsonencode(
        [
            "created",
        ]
    )
    config     = jsonencode(
        {
            created-by = "Elvis Presley"
            name       = "test-terraform-test-43"
            where      = "gracelands"
        }
    )
    dynamic  = jsonencode(
        {
            created = "21/10/2020 19:27:25"
        }
    )
}
```
 
In the executor script the `created` field is returned just like the others. No extra handling is required:

```python
if event == "create":
    # Create a unique file /tmp/json_file.pyXXXX and write the data to it
    . . .
    input_dict["created"] = datetime.now().strftime("%d/%m/%Y %H:%M:%S")
 
```

#### Referencing in TF template

This an example how to reference the resource and access its attributes

Let's say your script returned the following result
 
```json
{
  "id": "my-123",
  "name": "my-resource",
  "capacity": "20g"
}
```

then the resource in TF will have these attributes

```hcl
id = "my-123"
config = jsonencode(({
  name = "my-resource"
  capacity = "20g"
})
```

you can access these attributes using variables and the jsondecode function:

```hcl-terraform
${multiverse_custom_resource.my_custom_resource.id} # accessing id
${jsondecode(multiverse.myresource.config)["name"]}
${jsondecode(multiverse.myresource.config)["capacity"]}
```

#### Why the attribute *config* is JSON?

This will give you flexibility in passing your arguments with mixed types. We couldn't define a with generic mixed types, 
if we used map then all attributes have to be explicitly defined in the schema or all its attributes have the same type.

## Writing an Executor Script

A executor script must accept a single argument (the event), it must read a single 
JSON expression from its standard input and output one on stdout. The script must be able to handle the TF event and the JSON payload *config*

#### Input

* `event` : will have one of these values `create, read, delete, update, exists`
* `config` : is passed via `stdin`

Provider configuration data is passed in these environment variables:

* `id` - if not `create` this is the ID of the resource as returned to Terraform in the create
* `script` - _as per the TF source files described above_.
* `id_key` - _as per the TF source files described above_.
* `executor` - _as per the TF source files described above_.
* `computed` - _as per the TF source files described above_.

The environment also contains attributes present in the `environment` section in the provider block.

#### Output
The `exists` event expects either `true` or `false` on the stdout of the execution. 
`delete` sends nothing on stdin and requires no output on stdout.
The other events require JSON on the standard output matching the input JSON plus any dynamic fields.
The `create` execution must have the id of the resource in the field named by the `id_key` field.

#### Example

Your script could look something like the `json_file` example below. This script maintains files in the file system 
containing JSON data in the `config` field. The created datetime is returned as a dynamic field. 

```python
import os
import sys
import json
import tempfile
from datetime import datetime

if __name__ == '__main__':
   result = None
   event = sys.argv[1] # create, read, update or delete, maybe exists too

   id = os.environ.get("filename")            # Get the id if present else None
   script = os.environ.get("script")

   if event == "exists":
      # ignore stdin
      # Is file there?
      if id is None :
          result = False
      else:
          result = os.path.isfile(id)
      print('true' if result else 'false')
      exit(0)

   elif event == "delete":
        # Delete the file
        os.remove(id)
        exit(0)

   # Read the JSON from standard input
   input = sys.stdin.read()
   input_dict = json.loads(input)

   if event == "create":
        # Create a unique file /tmp/json_file.pyXXXX and write the data to it
        ff = tempfile.NamedTemporaryFile(mode = 'w+',  prefix=script, delete=False)
        input_dict["created"] = datetime.now().strftime("%d/%m/%Y %H:%M:%S")
        ff.write(json.dumps(input_dict))
        ff.close()
        input_dict.update({ "filename" : ff.name}) # Give the ID back to Terraform - it's the filename
        result = input_dict

   elif event == "read":
        # Open the file given by the id and return the data
        fr=open(id, mode='r+')
        data = fr.read()
        fr.close()
        if len(data) > 0:
            result = json.loads(data)
        else:
            result = {}

   elif event == "update":
       # write the data out to the file given by the Id
       fu=open(id,mode='w+')
       fu.write(json.dumps(input_dict))
       fu.close()
       result = input_dict

   print(json.dumps(result))

```

To test your script before using in TF, just give it JSON input and environment variables. You can also use the test harnesses
of your development language.

```bash
echo "{\"key\":\"value\"}" | id=testid001 python3 my_resource.py create
```
## Renaming the Resource Type

In your Terraform source code you may not want to see the resource type `multiverse`. You might a 
better name, reflecting the actual resource type you're managing. So you might want this instead:

```hcl-terraform
resource "spot_io_elastic_instance" "myapp" {
  provider = "multiverse"
  executor = "python3"
  script = "spotinst_mlb_targetset.py"
  id_key = "id"
  config = jsonencode({        
         // . . .
        })
}
```
The added `provider =` statement forces Terraform to use the multiverse provider for the resource. 

## Adding Resource Types

You can configure multiple resource types for the same provider, such as:

```hcl-terraform
resource "multiverse_database" "myapp" {
  config = jsonencode({        
         // . . .
        })
}
resource "multiverse_network" "myapp" {
  config = jsonencode({        
         // . . .
        })
}

```
  
We need to tell the provider which resource types it is providing to Terraform. By default, the only resource type
it provides is the `multiverse` type. To enable other names set the environment variable 'TERRAFORM_MULTIVERSE_RESOURCETYPES' 
include the resource type names in a space-separated list such as this:
```shell script
export TERRAFORM_MULTIVERSE_RESOURCETYPES='database network'
```
### Multiple Provider Names and Resource Types
If you have duplicated the provider (see 'Renaming the Provider') you can still use the RESOURCETYPES variable name. 
It is of the form: `TERRAFORM_{providername upper case}_RESOURCETYPES`. Hence you can use the new name. e.g.
```shell script
export TERRAFORM_LINUX_RESOURCETYPES='json_file network_interface directory'
```

## Configuring the Provider

Terraform allows [configuration of providers](https://www.terraform.io/docs/configuration/providers.html#provider-configuration-1), 
in a `'provider` clause. The multiverse provider also has fields where you specify the default executor, script and id fields.  
An additional field `environment` contains a map of environment variables which are passed to the script when it is executed. 

This means you don't need to repeat the `executor` nad `script` each time you use the provider.  You can 
override the defaults in the resource block as below.

```hcl-terraform
provider "multiverse" {
  environment = {
    servername = "api.example.com"
    api_token = "redacted"
  }
  executor = "python3"
  script = "json_file.py"
  id_key = "id"
}

resource "multiverse_alpha" "h1" {
  config = jsonencode({
      "name": "test-terraform-test-1",
    })
}

resource "multiverse_alpha" "h2" {
  script = "hello_world_v2.py"
  config = jsonencode({
      "name": "test-terraform-test-2",
    })
}

```

## Renaming the Provider

From the Terraform manual:

> Resource names are nouns, since resource blocks each represent a single object Terraform is managing. Resource names must always start with their containing provider's name followed by an underscore, so a resource from the provider postgresql might be named postgresql_database.

You can rename the provider itself. This could be to 'fake out' a normal provider to investigate its behaviour or 
emulate a defunct provider. 

Or maybe you just want a name you prefer. 

This can be achieved by copying or linking to the provider binary file with a 
name inclusive of the provider name:

```shell script
 # Move to the plugins directory wherein lies the provider
cd ~/.terraform.d/plugins/github.com/mobfox/alpha/0.0.1/linux_amd64
# Copy the original file
cp terraform-provider-multiverse  terraform-provider-spot_io_elastic_instance
# or maybe link it
ln -s terraform-provider-multiverse  terraform-provider-spot_io_elastic_instance
```

Then you need to configure the provider in your TF file:

```hcl-terraform
terraform {
  required_version = ">= 0.13.0"
  required_providers {
    spot_io_elastic_instance = {
      source = "github.com/mobfox/spot_io_elastic_instance"
      version = ">=0.0.1"
    }
  }
}
```
How does this work? The provider extracts the name of the provider from its own executable. By default, the multiverse provider sets the default resource type
to the same as the provider name.  

#### Renaming the Provider in Test or Debuggers

When a test harness or debugger uses a random name for the provider, you can override this with the environment variable `TERRAFORM_MULTIVERSE_PROVIDERNAME`. as in:

```shell script
$ export TERRAFORM_MULTIVERSE_PROVIDERNAME=multiverse
```

## Using the Embedded JavaScript Interpreter

As well as sub-process `executors`, multiverse also includes the [Otto JavaScript Interpreter](https://github.com/robertkrimen/otto). 
This means you can code in JavaScript without needing an external language or program. 
Since Otto does not have file handling or networking functions it is not suitable for writing resource providers - but this may change.
The `javascript` field identifies the JavaScript source file to run from the current directory:

````hcl-terraform
provider "multiverse" {
  id_key = "id"
  computed = jsonencode([
    "created"])
  javascript = "echo.js"
}
````
When the provider starts the interpreter starts it sets these global variables:

* variable defined by `id_key` is set to the resource's id
* variable `config` is set to the resource config (unpacked)
* variable `event` is set to the terraform event (create, update etc)
* variable `id_key`
* variable `javascript`
* variable `computed`

The last statement of the script must evaluate to a map of strings containing the resource configuration in "config" 
for example in this script we echo the incoming config on the last line.

````javascript
if (event == "create") {
    config[id_key] = "42";
    config["created"] = new Date().toLocaleString();
}
config
````

If the event is `create` the new ID must be returned in the `config[id_key]` field.

Otto may be run standalone via the `otto` command.

Refer to the Otto documentation for facilities of the JavaScript interpreter. 








## Developing the Provider


If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (version 1.15.2+ is *required*). 
You'll also need to correctly setup a [GOPATH](http://golang.org/doc/code.html#GOPATH), as well as adding `$GOPATH/bin` to your `$PATH`.

 > A good IDE is always beneficial. The kindly folk at [JetBrains](https://www.jetbrains.com/) provide Open Source authors with a free licenses to their excellent [Goland](https://www.jetbrains.com/go/) product, a cross-platform IDE built specially for Go developers   

To compile the provider, run `make build`. This will build the provider and put the provider binary in the workspace directory.

```sh script
$ make build
```

In order to test the provider, you can simply run `make test`.

```sh
$ make test
```

To install the provider in the usual places for the `terraform` program, run `make install`. It will place it the plugin directories:

```
$HOME/.terraform.d/
└── plugins
    ├── github.com
    │   └── mobfox
    │       ├── alpha
    │       │   └── 0.0.1
    │       │       └── linux_amd64
    │       │           └── terraform-provider-alpha
    │       └── multiverse
    │           └── 0.0.1
    │               └── linux_amd64
    │                   ├── terraform-provider-alpha
    │                   └── terraform-provider-multiverse
    ├── terraform-provider-alpha
    └── terraform-provider-multiverse
```


Feel free to contribute!
