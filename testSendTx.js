const Caver = require('caver-js')
const caver= new Caver('http://52.78.52.111:8551')

const privateKeys = [
    '0x50e57534f668886dc6742912169a4bebf497cad69ab74eca4b4e1392268c42cb', 
    '0x3f6a32dcd80c9b2ed84182805265d635ec65c364d4fe181a1932459db49e2850', 
    '0x25d183984cb67440259cca8671f06d2e1a03025ec398903fb4eea8b8914bbd47',
    '0x2c3e3ea93542ae3a8ba907b6bad49dbbb2c5a7750220301f8634d0b3a07af340'
]

const nodeKey = `0xacd28c553ada54b9266b7fa022cf351c19387f42d1fd2d171a3acae8baaafead`

async function decodeTx() {
    const raw = ''
    tx = caver.transaction.decode(raw)
    console.log(tx)
}

function makeAccount() {
    for(let i=0; i < 4; i++) {
        const keyring = caver.wallet.keyring.generate();
        console.log(`private key : ${keyring.key.privateKey}, address : ${keyring.address}`)
    }
}

async function fillBalance() {
    const nodeKeyring = caver.wallet.keyring.createFromPrivateKey(nodeKey)
    caver.wallet.add(nodeKeyring)
    

    for(let i=0; i<privateKeys.length; i++) {
        const keyring = caver.wallet.keyring.createFromPrivateKey(privateKeys[i])
        const vt = caver.transaction.valueTransfer.create({
            from : nodeKeyring.address,
            to : keyring.address, 
            value : caver.utils.toPeb(100, 'KLAY'),
            gas : 25000
        })
        // Sign to the transaction
	    const signed = await caver.wallet.sign(nodeKeyring.address, vt)

        // Send transaction to the Klaytn blockchain platform (Klaytn)
        // caver.rpc.klay.sendRawTransaction(signed).on('transactionHash', h => {
        //     console.log(h)
        // })
        receipt = await caver.rpc.klay.sendRawTransaction(signed)
        console.log(receipt)
    }
}

async function makeTxs() {
    let infoList = []

    for(let i=0; i < privateKeys.length; i++) {
        const keyring = caver.wallet.keyring.createFromPrivateKey(privateKeys[i])
        nonce = await caver.rpc.klay.getTransactionCount(keyring.address)
        let info = []
        for(let j=0; j < 5; j++) {
            const vt = caver.transaction.valueTransfer.create({
                from : keyring.address,
                to : '0x8084fed6b1847448c24692470fc3b2ed87f9eb47', 
                value : caver.utils.toPeb(1, 'ston'),
                gas : 25000,
                nonce : nonce
            })
            
            const signed = await caver.wallet.sign(keyring.address, vt)
            nonce++;

            info[j] = {txHash : signed.getTransactionHash(), raw : signed.getRawTransaction()}
        }
        infoList[i] = info
    }

    return infoList
}

async function sendTxs() {
    txs = []
    
    for(let i=0; i<privateKeys.length; i++) {
        const keyring = caver.wallet.keyring.createFromPrivateKey(privateKeys[i])
        caver.wallet.add(keyring)
    }

    info = await makeTxs()
    
    let index = 0;
    for(let i=0; i<5; i++) {
        for(let j=0; j<4; j++) {
            txs[index++] = info[j][i]
        }
    }

    for(let i=0; i<txs.length; i++) {
        caver.rpc.klay.sendRawTransaction(txs[i].raw).on('transactionHash', hash => {
            console.log(hash)
        })
    }
}

//fillBalance()
sendTxs();