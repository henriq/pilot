// Shared logic for the Cleanup prereleases workflow.
// Invoked from both the `smoke` job (dryRun=true, preview on PRs) and the
// `cleanup` job (always dryRun=false for real execution).

module.exports = async ({ github, context, core, dryRun = true }) => {
  const { owner, repo } = context.repo;

  const releases = await github.paginate(
    github.rest.repos.listReleases,
    { owner, repo, per_page: 100 }
  );

  const prereleases = releases
    .filter(r => r.prerelease)
    .sort((a, b) => {
      const timeDiff = new Date(b.published_at || b.created_at) - new Date(a.published_at || a.created_at);
      return timeDiff !== 0 ? timeDiff : b.id - a.id;
    });

  if (prereleases.length <= 1) {
    console.log(dryRun ? '[dry-run] No prereleases would be deleted' : 'No old prereleases to delete');
    return { kept: prereleases[0]?.tag_name ?? null, deleted: 0, failed: 0, remaining: 0, dryRun };
  }

  const toDelete = prereleases.slice(1, 51);
  console.log(`Keeping latest prerelease: ${prereleases[0].tag_name}`);
  if (prereleases.length > 51) {
    core.warning(`Backlog of ${prereleases.length - 1} prereleases exceeds per-run cap of 50; ${prereleases.length - 51} will remain after this run`);
  }
  console.log(`${dryRun ? '[dry-run] Would delete' : 'Deleting'} ${toDelete.length} old prerelease(s)`);

  const isBenign = (e) => e.status === 404 || e.status === 422;
  let deleted = 0;
  let failed = 0;

  for (const release of toDelete) {
    if (dryRun) {
      console.log(`[dry-run] Would delete: ${release.tag_name}`);
      continue;
    }

    try {
      try {
        console.log(`Deleting release: ${release.tag_name}`);
        await github.rest.repos.deleteRelease({ owner, repo, release_id: release.id });
      } catch (e) {
        if (isBenign(e)) {
          core.warning(`Release ${release.tag_name} already deleted or not found`);
        } else {
          failed++;
          core.error(`Failed to delete release ${release.tag_name} (status ${e.status}): ${e.message}`);
        }
        continue;
      }

      try {
        await github.rest.git.deleteRef({ owner, repo, ref: `tags/${release.tag_name}` });
        console.log(`Deleted tag: ${release.tag_name}`);
      } catch (e) {
        if (isBenign(e)) {
          core.warning(`Tag ${release.tag_name} already deleted or not found`);
        } else {
          failed++;
          core.error(`Failed to delete tag ${release.tag_name} (status ${e.status}): ${e.message}`);
        }
      }

      deleted++;
    } finally {
      // Throttle to stay under GitHub's secondary rate limit on mutating calls.
      await new Promise(r => setTimeout(r, 250));
    }
  }

  const remaining = prereleases.length > 51 ? prereleases.length - 51 : 0;
  const reportedDeleted = dryRun ? toDelete.length : deleted;
  await core.summary
    .addHeading(`Prerelease Cleanup${dryRun ? ' (dry-run)' : ''}`)
    .addList([
      `Kept: ${prereleases[0].tag_name}`,
      `${dryRun ? 'Would delete' : 'Deleted'}: ${reportedDeleted} prerelease(s)`,
      `Failed: ${failed}`,
      `Remaining backlog after run: ${remaining}`,
    ])
    .write();

  if (failed > 0) {
    core.setFailed(`${failed} delete operation(s) failed`);
  }

  return { kept: prereleases[0].tag_name, deleted: reportedDeleted, failed, remaining, dryRun };
};
