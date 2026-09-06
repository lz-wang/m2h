// Reader footer attribution: "Powered by m2h <version>" closes every
// successfully opened document. The repository link opens the project; the
// version link opens the matching release for x.y.z versions, or the release
// list for development builds (dev-<date>-<sha>). This replaces the sidebar
// footer's GitHub + version pair — the link semantics are unchanged, only the
// placement moved from the navigation shell into the reader canvas, so
// single-file previews (which render no sidebar) carry the attribution too.
const repositoryURL = "https://github.com/lz-wang/m2h";

const releaseVersionPattern = /^\d+\.\d+\.\d+$/;

function releaseURL(version: string): string {
  if (releaseVersionPattern.test(version)) {
    return `${repositoryURL}/releases/tag/v${version}`;
  }
  return `${repositoryURL}/releases`;
}

export function ReaderFooter({ version }: { version: string }) {
  return (
    // No contentinfo role here, even though SidebarInset's <main> strips the
    // footer's implicit one: this is a per-document attribution, not the
    // application's global footer, so generic is exactly the right semantics
    // — contentinfo is reserved for a top-level landmark.
    <footer className="reader-footer">
      Powered by{" "}
      <a href={repositoryURL} target="_blank" rel="noreferrer">
        m2h
      </a>
      {version !== "" ? (
        <>
          {" "}
          <a href={releaseURL(version)} target="_blank" rel="noreferrer">
            {version}
          </a>
        </>
      ) : null}
    </footer>
  );
}
