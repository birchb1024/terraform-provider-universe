# Terraform Provider Universe

You can use this provider instead of writing your own Terraform Custom Provider in the Go language. Just write your 
logic in any language you prefer (python, node, java, shell) and use it with this provider. You can write a script that
will be used to create, update or destroy external resources that are not supported by Terraform providers. 

## Maintainers

terraform-provider-universe is a fork of the multiverse provider. It is maintained by Peter Birch (birchb1024). 
This fork is no longer compatible with the original hence it is renamed 'universe'

The MobFox DevOps team at [MobFox](https://www.mobfox.com/) maintains the original 'multiverse' provider. 

##Requirements

-	[Terraform](https://www.terraform.io/downloads.html) 1.0
-	[Go](https://golang.org/doc/install) 1.16.5 (to build the provider plugin)

## Installing the Provider

You can download binary versions in GitHub [here](https://github.com/birchb1024/terraform-provider-universe/releases)

Otherwise, if you're a Golang user, you can `get` with

```shell script
    (cd /tmp; GO111MODULE=on go get github.com/birchb1024/terraform-provider-universe)
```

The [installer script](https://github.com/birchb1024/terraform-provider-universe/blob/master/scripts/install.sh) places
 the binary in the correct places to be picked up by Terraform `init`. Alternatively you can copy the `terraform-provider-universe` 
 file into the directories, ensuring the file mode is executable. Here's the layout:

```
~/.terraform.d/plugins/
├── github.com
│   └── birchb1024
│       └── universe
│           └── 0.0.5
│               └── linux_amd64
│                   └── terraform-provider-universe
└── terraform-provider-universe
```

# Using the provider

Remember to `terraform init` in your working directory so the new provider is found. Check with `terraform providers`

Check the `examples/json_file` directory

Here an example of a provider which creates json files in /tmp and stores configuration data in it. 
This is implemented in the json_file example directory.

## Example TF

Here's a TF which creates three JSON files in /tmp.

```hcl
//
// This example needs environment variables to specify resource types:
//
//   export TERRAFORM_UNIVERSE_RESOURCETYPES='json_file'
//   export TERRAFORM_LINUX_RESOURCETYPES='json_file'
//
terraform {
  required_version = ">= 0.13.0"
  required_providers {
    universe = {
      source = "github.com/birchb1024/universe"
      version = ">=0.0.5"
    }
    linux = {
      source = "github.com/birchb1024/linux"
      version = ">=0.0.5"
    }
  }
}
provider "universe" {
  executor = "python3"
  script = "json_file.py"
  id_key = "filename"
  environment = {
    api_token = "redacted"
    servername = "api.example.com"
    api_token = "redacted"
  }
}

resource "universe_json_file" "h" {
  config = jsonencode({
    "name": "Don't Step On My Blue Suede Shoes",
    "created-by" : "Elvis Presley",
    "where" : "Gracelands"
    "hit" : "Gold"
    "@created": 23
  })
}

resource "universe_json_file" "hp" {
  config = jsonencode({
    "name": "Another strange resource",
    "main-character" : "Harry Potter",
    "nemesis" : "Tom Riddle",
    "likes" : [
      "Ginny Weasley",
      "Ron Weasley"
    ],
    "@created": 23
  })
}

resource "linux_json_file" "i" {
  executor = "python3"
  script = "json_file.py"
  id_key = "filename"
  config = jsonencode({
    "name": "Fake strange resource"
  })
}

output "hp_name" {
  value = jsondecode(universe_json_file.hp.config)["name"]
}

output "hp_created" {
  value = jsondecode(universe_json_file.hp.config)["@created"]
}
```

- When you run `terraform apply` the resource will be created / updated
- When you run `terraform destroy` the resource will be destroyed

### Attributes

* `executor (string)` could be anything like python, bash, sh, node, java, awscli ... etc
* `script (string)` the path to your script or program to run, the script must exit with code 0 and return a valid json string
* `id_key (string)` the key of returned result to be used as id by terraform
* `config (JSON string)` must be a valid JSON string. This contains the configuration of the resource and is managed by Terraform.

### Handling Dynamic Data from the Executor

The `config` field in the provider attributes is monitored by Terraform plan for changes because it is a Required field.
Terraform detects changes and puts them into the plan. However, your provider may generated attributes dynamically (such as the creation
date) of a resource. Precede the attribute with an at sign `@` and these fields will not be compared. If you're writing 
an executor script, you can return new @ fields. As follows:

```hcl-terraform
resource "json_file" "h" {
  provider = universe // because Terraform does not scan local providers for resource types.
  executor = "python3"
  script = "json_file.py"
  id_key = "filename"
  config = jsonencode({
      "name": "test-terraform-test-43",
      "created-by" : "Elvis Presley",
      "where" : "gracelands"
      "@created" : "unknown until apply"
    })
}
```

After the plan is applied the tfstate file will then contain information:

```hcl-terraform
resource "json_file" "h" {
    config     = jsonencode(
        {
            created-by = "Elvis Presley"
            name       = "test-terraform-test-43"
            where      = "gracelands"
            @created       = "28/10/2020 21:18:56"
        }
    )
    id     = "/tmp/json_file.pyearjouiw"
}
```
 
In the executor script the `@created` field is returned just like the others. No extra handling is required:

```python
if event == "create":
    # Create a unique file /tmp/json_file.pyXXXX and write the data to it
    . . .
    input_dict["@created"] = datetime.now().strftime("%d/%m/%Y %H:%M:%S")
 
```
### Configuring the Provider

Terraform allows [configuration of providers](https://www.terraform.io/docs/configuration/providers.html#provider-configuration-1), 
in a `'provider` clause. The universe provider also has fields where you specify the default executor, script and id fields.  
An additional field `environment` contains a map of environment variables which are passed to the script when it is executed. 

This means you don't need to repeat the `executor` nad `script` each time you use the provider.  You can 
override the defaults in the resource block as below.

```hcl-terraform
provider "universe" {
  environment = {
    servername = "api.example.com"
    api_token = "redacted"
  }
  executor = "python3"
  script = "json_file.py"
  id_key = "id"
}

resource "universe" "h1" {
  config = jsonencode({
      "name": "test-terraform-test-1",
    })
}

resource "universe" "h2" {
  script = "hello_world_v2.py"
  config = jsonencode({
      "name": "test-terraform-test-2",
    })
}

```

### Referencing in TF template

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
${universe_custom_resource.my_custom_resource.id} # accessing id
${jsondecode(universe.myresource.config)["name"]}
${jsondecode(universe.myresource.config)["capacity"]}
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

The environment also contains attributes present in the `environment` section in the provider block. That's good for
servernames and passwords which should not go via command-line arguments.

#### Output
The `exists` event expects either `true` or `false` on the stdout of the execution. 
`delete` sends nothing on stdin and requires no output on stdout.
The other events require JSON on the standard output matching the input JSON plus any dynamic fields.
The `create` execution must have the id of the resource in the field named by the `id_key` field.

#### Example 1

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
    event = sys.argv[1]  # create, read, update or delete, maybe exists too

    id = os.environ.get("filename")  # Get the id if present else None
    script = os.environ.get("script")

    if event == "exists":
        # ignore stdin
        # Is file there?
        if id is None:
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
        # Create a unique file /tmp/json-file.pyXXXX and write the data to it
        ff = tempfile.NamedTemporaryFile(mode='w+', prefix=script, delete=False)
        input_dict["@created"] = datetime.now().strftime("%d/%m/%Y %H:%M:%S")
        ff.write(json.dumps(input_dict))
        ff.close()
        input_dict.update({"filename": ff.name})  # Give the ID back to Terraform - it's the filename
        result = input_dict

    elif event == "read":
        # Open the file given by the id and return the data
        fr = open(id, mode='r+')
        data = fr.read()
        fr.close()
        if len(data) > 0:
            result = json.loads(data)
        else:
            result = {}

    elif event == "update":
        # write the data out to the file given by the Id
        fu = open(id, mode='w+')
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

#### Example 2

This example captures the use of `jsonfile` provider which is a renamed version of the `universe` provider.
First, follow the steps described in `Renaming the Provider` and create a new provider called `jsonfile` symlinking to `universe` provider.

Then, create a new folder and add the following content in `jsonfile.py`:

```python
#!/usr/bin/env python3
import os
import sys
import json
import tempfile
from datetime import datetime

if __name__ == '__main__':
    result = None
    event = sys.argv[1]  # create, read, update or delete, maybe exists too

    id = os.environ.get("filename")  # Get the id if present else None
    script = os.environ.get("script")

    if event == "exists":
        # ignore stdin
        # Is file there?
        if id is None:
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
        # Create a unique file /tmp/json-file.pyXXXX and write the data to it
        ff = tempfile.NamedTemporaryFile(mode='w+', prefix=script, delete=False)
        input_dict["@created"] = datetime.now().strftime("%d/%m/%Y %H:%M:%S")
        ff.write(input_dict["content"])
        ff.close()
        input_dict.update({"filename": ff.name})  # Give the ID back to Terraform - it's the filename
        result = input_dict

    elif event == "read":
        # Open the file given by the id and return the data
        fr = open(id, mode='r+')
        data = fr.read()
        fr.close()
        input_dict['content'] = data
        result = input_dict

    elif event == "update":
        # write the data out to the file given by the Id
        fu = open(id, mode='w+')
        fu.write(input_dict['content'])
        fu.close()
        result = input_dict

    print(json.dumps(result))
```

Then, define a simple terraform file e.g. `jsonfile.tf` which creates a JSON file whose content is maintained by the terraform file below:
```hcl
terraform {
  required_version = ">= 0.13.0"
  required_providers {
    jsonfile = {
      source = "github.com/birchb1024/jsonfile"
      version = ">=0.0.5"
    }
  }
}

resource "jsonfile" "h" {
  executor = "python3"
  script = "jsonfile.py"
  id_key = "filename"
  config = jsonencode({
    "name": "Don't Step On My Blue Suede Shoes",
    "created-by" : "Elvis Presley",
    "where" : "Gracelands",
    "hit" : "Gold",
    "@created": 23
    "content": "This is a test content2."
  })
}

output "hp_name" {
  value = jsondecode(jsonfile.h.config)["name"]
}
```

Deploy the changes via terraform:
```
terraform apply
```

## Renaming the Resource Type

In your Terraform source code you may not want to see the resource type `universe`. You might a 
better name, reflecting the actual resource type you're managing. So you might want this instead:

```hcl-terraform
resource "spot_io_elastic_instance" "myapp" {
  provider = "universe"
  executor = "python3"
  script = "spotinst_mlb_targetset.py"
  id_key = "id"
  config = jsonencode({        
         // . . .
        })
}
```
The added `provider =` statement forces Terraform to use the universe provider for the resource. 

## Adding Resource Types

You can configure multiple resource types for the same provider, such as:

```hcl-terraform
resource "universe_database" "myapp" {
  config = jsonencode({        
         // . . .
        })
}
resource "universe_network" "myapp" {
  config = jsonencode({        
         // . . .
        })
}

```
  
We need to tell the provider which resource types it is providing to Terraform. By default, the only resource type
it provides is the `universe` type. To enable other names set the environment variable 'TERRAFORM_UNIVERSE_RESOURCETYPES' 
include the resource type names in a space-separated list such as this:
```shell script
export TERRAFORM_UNIVERSE_RESOURCETYPES='database network'
```
### Multiple Provider Names and Resource Types
If you have duplicated the provider (see 'Renaming the Provider') you can still use the RESOURCETYPES variable name. 
It is of the form: `TERRAFORM_{providername upper case}_RESOURCETYPES`. Hence you can use the new name. e.g.
```shell script
export TERRAFORM_LINUX_RESOURCETYPES='json_file network_interface directory'
```


## Renaming the Provider

You can rename the provider itself. This could be to 'fake out' a normal provider to investigate its behaviour or 
emulate a defunct provider. 

Or maybe you just want a name you prefer e.g. `spot_io_elastic_instance`

This can be achieved by copying or linking to the provider binary file with a 
name inclusive of the provider name:

```shell script
# Move to the author directory
cd ~/.terraform.d/plugins/github.com/birchb1024/
# Create the sub-folders with same name upto the provider binary and traverse to it
mkdir -p spot_io_elastic_instance/0.0.5/linux_amd64
cd spot_io_elastic_instance/0.0.5/linux_amd64
# Create a link to the binary
ln -s ~/.terraform.d/plugins/github.com/birchb1024/universe/0.0.5/linux_amd64/terraform-provider-universe terraform-provider-spot_io_elastic_instance
# OR, copy it
cp ~/.terraform.d/plugins/github.com/birchb1024/universe/0.0.5/linux_amd64/terraform-provider-universe terraform-provider-spot_io_elastic_instance
```

Alternatively, if you have the source repository checked out, the installer script will add a second provider:

```shell script
$ ./scripts/install.sh spot_io_elastic_instance
```

Then you need to configure the provider in your TF file:

```hcl-terraform
terraform {
  required_version = ">= 0.13.0"
  required_providers {
    spot_io_elastic_instance = {
      source = "github.com/birchb1024/spot_io_elastic_instance"
      version = ">=0.0.5"
    }
  }
}
```
How does this work? The provider extracts the name of the provider from its own executable. By default, the universe provider sets the default resource type
to the same as the provider name.  


#### Renaming the Provider in Test or Debuggers

When a test harness or debugger uses a random name for the provider, you can override this with the environment variable `TERRAFORM_UNIVERSE_PROVIDERNAME`. as in:

```shell script
$ export TERRAFORM_UNIVERSE_PROVIDERNAME=universe
```

## Building The Provider

Clone repository to: `$GOPATH/src/github.com/birchb1024/terraform-provider-universe`

```sh
$ mkdir -p $GOPATH/src/github.com/birchb1024; cd $GOPATH/src/github.com/birchb1024
$ git clone git@github.com:birchb1024/terraform-provider-universe.git
```

Enter the provider directory and build the provider

```sh
$ cd $GOPATH/src/github.com/birchb1024/terraform-provider-universe
$ make build
```

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
    │   └── birchb1024
    │       └── universe
    │           └── 0.0.5
    │               └── linux_amd64
    │                   └── terraform-provider-universe
    └── terraform-provider-universe
```

Feel free to contribute!
