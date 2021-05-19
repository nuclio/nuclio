module.exports = async ({github, context, prNumber, labelsToAdd, labelsToRemove}) => {

    console.log(`Adding labels ${labelsToAdd} to PR #${prNumber}`)
    await github.issues.addLabels({
        issue_number: prNumber,
        owner: context.repo.owner,
        repo: context.repo.repo,
        labels: labelsToAdd
    })

    console.log(`Removing labels ${labelsToRemove} from PR #${prNumber}`)
    await Promise.all(labelsToRemove.map(labelName => github.issues.removeLabel({
        issue_number: prNumber,
        owner: context.repo.owner,
        repo: context.repo.repo,
        name: labelName
    }).catch(error => {
        if (error.toString().toLowerCase().includes('label does not exist')) {
            console.log(`Ignoring not existing label error: ${error.toString()}`)
            return
        }
        throw error
    })))
}
