// Deliberately unsafe/broken fixture — exercises every jvm.js finding id.
//
// Package-private (no `public`) on purpose: dirty.java and clean.java share
// a directory, and only a public top-level class must match its filename.
// Neither file may declare a class with the same name as the other.
import java.io.ObjectInputStream;
import java.security.MessageDigest;
import java.sql.Statement;
import java.util.Random;
import javax.xml.parsers.DocumentBuilderFactory;

class DirtyExamples {

    void lookupUser(Statement stmt, String id) throws Exception {
        stmt.executeQuery("SELECT * FROM t WHERE id = " + id);
    }

    void deserialize(java.io.InputStream payload) throws Exception {
        ObjectInputStream in = new ObjectInputStream(payload);
        in.readObject();
    }

    void parseXml() throws Exception {
        DocumentBuilderFactory.newInstance();
    }

    void hashPassword(String password) throws Exception {
        MessageDigest.getInstance("MD5");
    }

    String makeToken() {
        String token = String.valueOf(new Random().nextLong());
        return token;
    }

    void insecureTrust() {
        boolean trustAllCerts = TrustAllCerts.INSTANCE != null;
    }

    void swallow() {
        try {
            go();
        } catch (Exception e) {
        }
    }

    void printOnly() {
        try {
            go();
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    void debugPrint(int x) {
        System.out.println("here " + x);
    }

    void runGitLog(String branch) throws Exception {
        Runtime.getRuntime().exec("git log " + branch);
    }

    void runShell(String cmd) throws Exception {
        new ProcessBuilder("sh", "-c", cmd).start();
    }

    void go() throws Exception {
    }
}
