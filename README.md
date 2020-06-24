# Terraspec

Terraspec is a unit test framework for terraform configurations. It lets you quickly check what your terraform configurations will create for a specific set of input variables.

Terraspec is an additional tool to your terraform toolbox. It doesn't provision any resource in your cloud provider nor does it let you test your cloud resources. Terraspec only tests the code you write in your config.

For those aware of Chef development, `terraspec` aims to be an equivalent to `chefspec` rather than `test-kitchen`.

## Usage

### Write your tests

In your terraform configuration directory, create a `spec` folder and a subfolder for every test scenario.

At least, your test suite subfolder must contain a `.tfspec` file containing all the assertions on your code. 
To test a different scenario than the  default input variables, you can provide a `.tfvars` file as well.

**Examples are available in the `examples` directory of this repository.**

Writing an assertion is as easy as writing your initial terraform configuration. If you want to check the behaivor of this terraform code :

```
resource "aws_instance" "my-server" {
    ami = var.ami
    ...
}
```

You can write the following code in your `tfspec` file : 
```
assert "aws_instance" "my-server" {
    ami = "the-ami-value-expected"
}
```

To test the value of an output, you can write :
```
assert "output" "output-name" {
    value = "expected-output-value"
}
```


### Run 

To call `terraspec`, you must have run `terraform init` first to have all the plugins and modules downloaded. 
The first time you write `terraspec` should be like : 
```
$ terraform init -backend=false
$ terraspec 
```
As terraspec will never try to read your current state, you don't even need to init the remote backend.

If you want to run a single test scenario, you can specify it with the `--spec` flag : 
```
$ terraspec --spec spec/my-scenario
```

The command line flag `--diplay-plan` can help to write your tests. As name suggests, with this flag `terraspec` will print you the output of `terraform plan`. 


## Use cases

The examples given so far are really easy and can seem useless. However there a re situations where writing this kind of tests is really helpful :

- When your configuration do tricky operations with terraform provided function like splitting a dns name to find the DNS zone to update, or doing computation on an IP address or subnet mask, ...
- When your configuration may or may not create additionnal resources, depending on advanced conditions
- When you want to check all your tags have been correctly merged and applied to your resources
- ...

Also, as a module author, it can be interesting to ensure the new version of your module won't have side effects so users can update peacefully.

## How it works

Terraspec embeds terraform code, so even if it doesn't make call to the `terraform` command, it relies on `terraform` to compute the plan. Nevertheless, `terraspec` wraps all calls to the underlying plugin so that the terraform state is never read, nor the `data` resource.
This makes `terraspec` able to validate any configuration, whichever cloud provider you use, without any credentials to that cloud provider.

## Limitations

Terraspec is still at its early stages and doesn't cover all cases yet. Here are the known limitations identified so far.

### Data resources

Currently, terraspec doesn't support mocking the `data` blocks. All datasources will return empty results. For certain configuration, this can make the computation of the plan fail and `terraspec` won't be able to test your code.

The current workaround is for your plan to have `input` variables (default value being empty) that you override in your test scenario. In your plan, using `coalesce(var.mock_result, data.my_resource.property)` will make tests works while keeping the datasource the default source of truth.


### Negative assertions

Checking a resource is created with correct parameters is currently supported. Checking a resource is **not** created, or checking a created resource **doesn't** contain a specific `block` is under development.

### Installation

No pre-built binaries are available yet. `go get github.com/nhurel/terraspec` is the easiest way to get started

## License

Mozilla Public License 2.0




