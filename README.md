## TODO

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

必須ヘッダ周りは基本的にTransaction以下で処理させる。
Viaヘッダは完全にTUより下でしか変更できないようにする。
(ViaをどうしてもいじりたいならSMMしてくれというスタンス)
- To *done*
- From *done*
- CSeq *done*
- Call-ID *done*
- Max-Forwards *done*
- Via *done*
- Contact

Via以外はメッセージ生成後にも場合によって書き換えれるようにはする。
けど、MessageのHeader属性に書いたものは無視されるようにする。


