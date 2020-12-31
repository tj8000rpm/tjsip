## TODO

### 雑多なやらないといけなこと

- Registerの処理
- INVITEの認証
- ルーティング処理
- 200 OK(INVITE)のACK対象となるダイアログが存在しない場合のチェック(現状別のトランザクションだからスルーしてしまう) / BYEも
   - 逆に 200 OK(INVITE) の再送にたいしてレスポンスコンテキストが存在しないため、転送されないのも同じ枠組みで修正。
   - 確立済のDialogじゃない場合に481返すようにすればOKかなと。
- WriteMessageしたときのエラーハンドル
- Viaはトランザクション以下で追加してるがレスポンスについては自分ででPop？いいか？


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


