## TODO

### 雑多なやらないといけなこと

- Viaはトランザクション以下で追加してるがレスポンスについては自分ででPop？いいか？
- MaxForwardチェック
- WriteMessageしたときのエラーハンドル
- 200 OK(INVITE)のACK対象となるダイアログが存在しない場合のチェック(現状別のトランザクションだからスルーしてしまう)


### For Signalling
- Handler customize
- Provide SIP parse helper functions

### For Carrier Grade Operation
- CDR
- Session Copy
- ToS
- Access Controll
    - IP address base controll
- Block / Unblock
    - handle block operation
    - Sorry page


## Memo

### レイヤリング
- TU(UAS/UAC/Proxy)
- Transaction(ST/CT)
- Transport(Server)

Via以外はメッセージ生成後にも場合によって書き換えれるようにはする。
けど、MessageのHeader属性に書いたものは無視されるようにする。


