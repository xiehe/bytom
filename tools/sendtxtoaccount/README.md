
# 发送远端交易
## Example
```
$ go build
$ export BYTOM_URL="http://192.168.199.62:9888"
$ ./sendtxttoaccount

```
available flags for `sendtxttoaccount`:

```
      --accountinfo string   acoount info(format: csv) (default "./accountinfo.csv")
      --config string        config file (default "./config.toml")
```
## Example for accountinfo.csv
```
sm1q3jqcknumx2lkrp00v8x92yq20j4j5huv3wkyd4,100
sm1qcldvvpxql3hk20nw2dq6jk2m4sum88lcsymq5a,200
sm1qttpx4lw4wfjrps2eamwk98fc3rez8waqykqsjz,300
sm1qyx6pr9r6fwh54jmf03cpwdjxqw8r0kjp5lxce7,400
sm1q55gy9h33w0ej3up7pdj82epsauchxa4jut6yjg,500

```

# config.toml
```
send_acct_id = "0D54P0N5G0A02"
send_asset_id = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
btm_gas=0.4
output_num=60
```
