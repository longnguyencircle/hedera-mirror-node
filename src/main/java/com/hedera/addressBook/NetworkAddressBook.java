package com.hedera.addressBook;

import java.io.FileNotFoundException;
import java.io.IOException;
import java.util.Map;

import com.hedera.configLoader.ConfigLoader;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;
import org.apache.logging.log4j.Marker;
import org.apache.logging.log4j.MarkerManager;

import com.hedera.hashgraph.sdk.Client;
import com.hedera.hashgraph.sdk.HederaException;
import com.hedera.hashgraph.sdk.HederaNetworkException;
import com.hedera.hashgraph.sdk.account.AccountId;
import com.hedera.hashgraph.sdk.crypto.ed25519.Ed25519PrivateKey;
import com.hedera.hashgraph.sdk.file.FileId;

import io.github.cdimascio.dotenv.Dotenv;

import com.hedera.hashgraph.sdk.file.FileContentsQuery;

import java.io.FileOutputStream;

/**
 * This is a utility file to read back service record file generated by Hedera node
 */
public class NetworkAddressBook {

	private static final Logger log = LogManager.getLogger("networkaddressbook");
	static final Marker LOGM_EXCEPTION = MarkerManager.getMarker("EXCEPTION");

    private static ConfigLoader configLoader = new ConfigLoader();
	private static String addressBookFile = configLoader.getAddressBookFile();

	static Client client;
	static Dotenv dotenv = Dotenv.configure().ignoreIfMissing().load();
    
    public static void main(String[] args) {

        var client = createHederaClient();

		log.info("Fecthing New address book from node {}.", dotenv.get("NODE_ADDRESS"));
        
        try {
            var contents = new FileContentsQuery(client)
                    .setFileId(new FileId(0, 0, 102))
                    .execute();

            writeFile(contents.getFileContents().getContents().toByteArray());
        } catch (FileNotFoundException e) {
    		log.error(LOGM_EXCEPTION, "Address book file {} not found.", addressBookFile);
        } catch (IOException e) {
    		log.error(LOGM_EXCEPTION, "An error occurred fetching the address book file: {} ", e);
        } catch (HederaNetworkException e) {
    		log.error(LOGM_EXCEPTION, "An error occurred fetching the address book file: {} ", e);
		} catch (HederaException e) {
    		log.error(LOGM_EXCEPTION, "An error occurred fetching the address book file: {} ", e);
		}
		log.info("New address book successfully saved to {}.", addressBookFile);
		System.out.println("");
		System.out.println("**********************************************************************************");
		System.out.println("***** New address book successfully saved to " + addressBookFile);
		System.out.println("**********************************************************************************");
		System.out.println("");
	}

    public static void writeFile(byte[] newContents) throws IOException {
        FileOutputStream fos = new FileOutputStream(addressBookFile);
        fos.write(newContents);
        fos.close();
    }
    
	private static Client createHederaClient() {
	    // To connect to a network with more nodes, add additional entries to the network map
		
	    var nodeAddress = dotenv.get("NODE_ADDRESS","");
	    if (nodeAddress.isEmpty()) {
    		log.error(LOGM_EXCEPTION, "NODE_ADDRESS environment variable not set");
    		System.exit(1);
	    }
	    var client = new Client(Map.of(getNodeId(), nodeAddress));
	
	    // Defaults the operator account ID and key such that all generated transactions will be paid for
	    // by this account and be signed by this key
	    client.setOperator(getOperatorId(), getOperatorKey());
	
	    return client;
	}
	
    public static AccountId getNodeId() {
    	try {
    		return AccountId.fromString(dotenv.get("NODE_ID"));
    	} catch (Exception e) {
    		log.error(LOGM_EXCEPTION, "NODE_ID environment variable not set");
    		System.exit(1);
    	}
    	return null;
    }

    public static AccountId getOperatorId() {
    	try {
    		return AccountId.fromString(dotenv.get("OPERATOR_ID"));
    	} catch (Exception e) {
    		log.error(LOGM_EXCEPTION, "OPERATOR_ID environment variable not set");
    		System.exit(1);
    	}
    	return null;
    	
    }
	
    public static Ed25519PrivateKey getOperatorKey() {
    	try {
    		return Ed25519PrivateKey.fromString(dotenv.get("OPERATOR_KEY"));
		} catch (Exception e) {
			log.error(LOGM_EXCEPTION, "OPERATOR_KEY environment variable not set");
			System.exit(1);
		}
		return null;
	    }
	
}


