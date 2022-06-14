# Thingsboard API Proy

A simple API proxy to make thingsboard managable by the [Terraform Restful Provider](https://github.com/magodo/terraform-provider-restful).

The concrete work includes:

1. Turns the authentication request from the Terraform Restful Provider in the urlencoded style, to its equivalent in body style, which is expected by the thingsboard API.
1. Turns `PUT` API request that is used by the Terraform Restful Provider to update a resource, to its equivalent `POST` form, which is expected by the thingsboard API.
