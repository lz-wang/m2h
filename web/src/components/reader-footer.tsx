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
    // role="contentinfo" is spelled out because a footer inside
    // SidebarInset's <main> loses its implicit contentinfo role, and that
    // landmark is what ties the attribution to the reader rather than the
    // navigation shell.
    <footer
      className="reader-footer"
      role="contentinfo"
      aria-label="m2h 项目信息"
    >
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
