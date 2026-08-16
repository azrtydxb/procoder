// Clean fixture — real near-misses for every rule jvm.js implements, all of
// which must stay silent.
//
// Package-private (no `public`) and named CleanExamples, distinct from
// DirtyExamples in dirty.java, which shares this directory — see the note
// there for why that matters.
import com.fasterxml.jackson.databind.ObjectMapper;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.sql.PreparedStatement;
import javax.xml.XMLConstants;
import javax.xml.parsers.DocumentBuilderFactory;

class CleanExamples {

    void lookupUser(PreparedStatement ps, String id) throws Exception {
        ps.setString(1, id);
        ps.executeQuery();
    }

    void deserialize(String json) throws Exception {
        ObjectMapper mapper = new ObjectMapper();
        mapper.readValue(json, Object.class);
    }

    void parseXml() throws Exception {
        DocumentBuilderFactory dbf = DocumentBuilderFactory.newInstance();
        dbf.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
        dbf.setXIncludeAware(false);
        dbf.setAttribute(XMLConstants.ACCESS_EXTERNAL_DTD, "");
    }

    void hashPassword(String password) throws Exception {
        MessageDigest.getInstance("SHA-256");
    }

    String makeToken() {
        byte[] buf = new byte[16];
        new SecureRandom().nextBytes(buf);
        return java.util.Base64.getEncoder().encodeToString(buf);
    }

    void handled() {
        try {
            go();
        } catch (Exception e) {
            log.error("failed", e);
            throw e;
        }
    }

    void debugPrint(int x) {
        log.info("here " + x);
    }

    void runGitLog(String branch) throws Exception {
        Runtime.getRuntime().exec(new String[]{"git", "log", branch});
    }

    void runShell(String cmd) throws Exception {
        new ProcessBuilder("git", "log", cmd).start();
    }

    void go() throws Exception {
    }
}
