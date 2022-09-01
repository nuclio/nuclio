/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
