#!/bin/bash
get_image()
{
    local PUBLISHED=$1
    local PROJECT_ID=$2
    local VERSION=$3
    local RHEL_API_KEY=$4

    if [[ $PUBLISHED == "published" ]]; then
        local PUBLISHED_FILTER="repositories.published==true"
    elif [[ $PUBLISHED == "not_published" ]]; then
        local PUBLISHED_FILTER="repositories.published!=true"
    else
        echo "Need first parameter as 'published' or 'not_published'." ; return 1
    fi

    local FILTER="filter=deleted==false;${PUBLISHED_FILTER};repositories.tags.name==${VERSION}"
    local INCLUDE="include=total,data.repositories.tags.name,data._id,data.container_grades.status"

    local RESPONSE=$( \
        curl --silent \
             --request GET \
             --header "X-API-KEY: ${RHEL_API_KEY}" \
             "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&${INCLUDE}")

    echo "${RESPONSE}"
}

wait_for_container_publish()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4
    local NOF_IMAGES=4

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 6 ))
    # Wait until the image is published
    for i in `seq 1 ${NOF_RETRIES}`; do
        local IS_PUBLISHED=$(get_image published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}" | jq -r '.total')

        if [[ $IS_PUBLISHED == "$NOF_IMAGES" ]]; then
            echo "Images are published, exiting."
            return 0
        else
            echo "Images are still not published, waiting..."
        fi

        if [[ $i == $NOF_RETRIES ]]; then
            echo "Timeout! Publish could not be finished"
            return 42
        fi
        sleep 10
    done
}

wait_for_container_unpublish()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4
    local NOF_IMAGES=4

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 6 ))
    # Wait until the image is unpublished
    for i in `seq 1 ${NOF_RETRIES}`; do
        local IS_NOT_PUBLISHED=$(get_image not_published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}" | jq -r '.total')

        if [[ $IS_NOT_PUBLISHED == "$NOF_IMAGES" ]]; then
            echo "Images are unpublished, exiting."
            return 0
        else
            echo "Images are still unpublishing, waiting..."
        fi

        if [[ $i == $NOF_RETRIES ]]; then
            echo "Timeout! Unpublishing could not be finished"
            return 42
        fi
        sleep 10
    done
}

checking_image_grade()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4
    local NOF_IMAGES=4
    FILTER="filter=deleted==false;repositories.tags.name==${VERSION}"

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
    for i in `seq 1 ${NOF_RETRIES}`; do

    local GRADE_PRESENCE=$(curl -s -X 'GET' \
      "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&page_size=100&page=0" \
      -H "accept: application/json" \
      -H "X-API-KEY: ${RHEL_API_KEY}" | jq -r -e '[.data[]? | select(.freshness_grades != null)] | length')

        if [[ ${GRADE_PRESENCE} -eq ${NOF_IMAGES} ]]; then
            GRADE_A=$(curl -s -X 'GET' \
            "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&page_size=100&page=0" \
            -H "accept: application/json" \
            -H "X-API-KEY: ${RHEL_API_KEY}" | jq -e '[.data[].freshness_grades[] | select(.grade == "A")] | length')
        if [[ ${GRADE_A} -eq ${NOF_IMAGES} ]]; then
            echo "The submitted images got a Health Index 'A'."
            return 0
        else
            echo "The submitted images didn’t get a Health Index 'A'."
            exit 1
        fi
        else
            echo "The submitted images still has unknown Health Index Image, waiting..."
        fi
        if [[ ${i} == ${NOF_RETRIES} ]]; then
            echo "Timeout! The submitted images has 'Unknown' Health Index."
            return 42
        fi
        sleep 20
    done
}

delete_container_image()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4
    FILTER="filter=deleted==false;repositories.published==true;repositories.tags.name==${VERSION}"
    MANIFEST_LIST_DIGEST=$(curl -X 'GET' --silent \
    -H "X-API-KEY: $RHEL_API_KEY" \
    -H 'Content-Type: application/json' \
    "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&page_size=100&page=0" | jq -r '.data | map(select(.container_grades.status == "completed" and (.creation_date | split("T")[0] == (now | strftime("%Y-%m-%d"))))) | last | select(.) | .repositories[0].manifest_list_digest')

    if [ -n "$MANIFEST_LIST_DIGEST" ]; then
        echo "Unpublishing certified images..."
        curl --request POST "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/requests/images" \
        -H 'content-type: application/json' \
        -H "X-API-KEY: $RHEL_API_KEY" \
        --data-raw '{"manifest_list_digest":"'$MANIFEST_LIST_DIGEST'","operation":"unpublish-manifest-list"}' \
        --compressed

        wait_for_container_unpublish $PROJECT_ID $VERSION $RHEL_API_KEY $TIMEOUT_IN_MINS

        echo "Deleting certified images..."
        curl --request POST "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/requests/images" \
        -H 'content-type: application/json' \
        -H "X-API-KEY: $RHEL_API_KEY" \
        --data-raw '{"manifest_list_digest":"'$MANIFEST_LIST_DIGEST'","operation":"delete-manifest-list"}' \
        --compressed
    else
        echo "No images to unpublish and delete."
    fi
}

# The function waits until all Elastic Load Balancers attached to EC2 instances (under the current Kubernetes context) are deleted. Takes a single argument - timeout.
wait_for_elb_deleted()
{
    local TIMEOUT_IN_MINS=$1
    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 6 ))
    INSTANCE_IDS=$(kubectl get nodes -o json | jq -r 'try([.items[].metadata.annotations."csi.volume.kubernetes.io/nodeid"][]|fromjson|."ebs.csi.aws.com"|select( . != null ))'| tr '\n' '|' | sed '$s/|$/\n/' | awk '{ print "\""$0"\""}')
    if [ ! -z "$INSTANCE_IDS" ]; then
        for i in `seq 1 ${NOF_RETRIES}`; do
            ACTIVE_ELB=$(aws elb describe-load-balancers | grep -E $INSTANCE_IDS >/dev/null; echo $?)
            if [ $ACTIVE_ELB -eq 1 ] ; then
               echo "Load Balancers are deleted."
               exit 0
            else
               echo "Load Balancers are still being deleting, waiting..."
            fi
            if [[ $i == $NOF_RETRIES ]]; then
                echo "Timeout! Deleting of Load Balancers couldn't be finished."
                return 42
            fi
            sleep 10
        done
    else
      echo "The required annotations 'csi.volume.kubernetes.io/nodeid' are missing in the EC2 instances metadata."
      exit 0
    fi
}

post_test_result()
{
    local PR_NUMBER=$1
    sudo apt-get update
    sudo apt-get install libxml2-utils
    # Define an array to store test run status
    declare -a test_run_status
    local failed_tests

    # Define an associative array to map report names to suite IDs
    declare -A suite_ids=(
        ["test_report_ee_01.xml"]="c27ecdc7f61258eff0f1de9e8de22e20"
    )

    # Initialize the table header
    success_comment="✅ All tests have passed\n--\n|| Total Tests | 🔴 Failures | 🟠 Errors | ⚪ Skipped |\n| :----: | :----: | :----: | :----: | :----: |\n"
    failed_comment="❌ Some tests failed\n--\n|| Total Tests | 🔴 Failures | 🟠 Errors | ⚪ Skipped |\n| :----: | :----: | :----: | :----: | :----: |\n"
    # Initialize the failed test section
    failed_test_block="\n<details><summary>Failed Tests</summary>\n\n|||\n| :----: | ---- |\n"

    # Loop through the test reports and generate a table row for each one
    for report in "${!suite_ids[@]}"
    do
        # Extract the relevant data from the test report using xmllint
        tests=$(xmllint --xpath 'string(//testsuites/testsuite/@tests)' "${GITHUB_WORKSPACE}/allure-results/pr/${report}")
        failures=$(xmllint --xpath 'string(//testsuites/testsuite/@failures)' "${GITHUB_WORKSPACE}/allure-results/pr/${report}")
        errors=$(xmllint --xpath 'string(//testsuites/testsuite/@errors)' "${GITHUB_WORKSPACE}/allure-results/pr/${report}")
        skipped=$(xmllint --xpath 'string(//testsuites/testsuite/@skipped)' "${GITHUB_WORKSPACE}/allure-results/pr/${report}")

        # Save status of the test run
        test_run_status+=("$([ "$failures" -gt 0 ] && echo true || echo false)" "$([ "$errors" -gt 0 ] && echo true || echo false)")

        if [ "$failures" -gt 0 ]; then
            failed_tests=$(xmllint --xpath "//testcase[@status='failed']/@name" "${GITHUB_WORKSPACE}/allure-results/pr/${report}" | cut -d '"' -f 2 |sed -n 's/.*\[It\] \(.*\) \[.*\]/<li>\1<\/li>/p' | tr '\n' ' ')
        fi

        # Get the suite ID from the array
        suite_id=${suite_ids[$report]}

        # Extract the substring "EE" or "OS" from the report name
        type="${report#test_report_}"
        type="${type%_01.xml}"

        # Construct a table row with a link to the test report
        link="${REPORT_PAGE_URL}/pr/${GITHUB_RUN_NUMBER}/#suites/${suite_id}"
        row="| [${type^^}](${link}) | $tests | $failures | $errors| $skipped |\n"
        failed_tests_row="| ${type^^} | $failed_tests |\n"

        # Append the row to the output
        success_comment+="$row"
        failed_comment+="$row"
        failed_test_block+="$failed_tests_row"
    done
    success_comment+="$failed_test_block"
    failed_comment+="$failed_test_block"
    # Send the output as a comment on the pull request using gh
    COMMENT_ID=$(gh api -H "Accept: application/vnd.github+json" /repos/hazelcast/hazelcast-platform-operator/issues/$PR_NUMBER/comments | jq '.[] | select(.user.login == "github-actions[bot]") | .id')
    if [[ $COMMENT_ID -ne "" ]]; then
        gh api --method DELETE -H "Accept: application/vnd.github+json" /repos/hazelcast/hazelcast-platform-operator/issues/comments/$COMMENT_ID
    fi
    if [[ "${test_run_status[*]}" == *"true"* ]]; then
        echo -e "$failed_comment" | gh pr comment ${PR_NUMBER} -F -
    else
        echo -e "$success_comment" | gh pr comment ${PR_NUMBER} -F -
    fi
}

# This function will restart all instances that are not in ready status and wait until it will be ready
wait_for_instance_restarted()
{
   local TIMEOUT_IN_MINS=$1
   local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
   NUMBER_NON_READY_INSTANCES=$(oc get machine -n openshift-machine-api -o json | jq -r '[.items[] | select(.status.providerStatus.instanceState | select(contains("running")|not))]|length')
   NON_READY_INSTANCE=$(oc get machines -n openshift-machine-api -o json | jq -r '[.items[] | select(.status.providerStatus.instanceState | select(contains("running")|not))]|.[].status.providerStatus.instanceId')
   if [[ ${NUMBER_NON_READY_INSTANCES} -ne 0 ]]; then
      for INSTANCE in ${NON_READY_INSTANCE}; do
         STOPPING_INSTANCE_STATE=$(aws ec2 stop-instances --instance-ids ${INSTANCE} | jq -r '.StoppingInstances[0].CurrentState.Name')
         echo "Stop instance $INSTANCE...The current instance state is $STOPPING_INSTANCE_STATE"
         aws ec2 wait instance-stopped --instance-ids ${INSTANCE}
         STARTING_INSTANCE_STATE=$(aws ec2 start-instances --instance-ids ${INSTANCE} | jq -r '.StartingInstances[0].CurrentState.Name')
         aws ec2 wait instance-running --instance-ids ${INSTANCE}
         echo "Starting instance $INSTANCE...The current instance state is $STARTING_INSTANCE_STATE"
         for i in `seq 1 ${NOF_RETRIES}`; do
            NUMBER_NON_READY_INSTANCES=$(oc get machine -n openshift-machine-api -o json | jq -r '[.items[] | select(.status.providerStatus.instanceState | select(contains("running")|not))]|length')
            if [[ ${NUMBER_NON_READY_INSTANCES} -eq 0 ]]; then
               echo "All instances are in 'Ready' status."
               return 0
            else
               echo "The instances restarted but are not ready yet. Waiting..."
            fi
            if [[ ${i} == ${NOF_RETRIES} ]]; then
               echo "Timeout! Restarted instances are still not ready."
               return 42
            fi
            sleep 20
         done
      done
   else
      echo "All instances are in 'Ready' status."
   fi
}

# The function merges all test reports (XML) files from each node into one report.
# Takes a single argument - the WORKFLOW_ID (kind,gke,eks, aks etc.)
merge_xml_test_reports() {
  sudo apt-get install -y xmlstarlet
  local WORKFLOW_ID=$1
  local files_by_edition=()
  local edition
  for file in $(ls ${GITHUB_WORKSPACE}/allure-results/$WORKFLOW_ID/test_report_*); do
         edition=$(basename "$file" | awk -F "_" '{print $3}')
         files_by_edition+=("$edition")
  done
  groups=$(printf "%s\n" "${files_by_edition[@]}" | awk '{print $1}' | sort -u)

  for group in $groups; do
      IFS=$'\n'
      local PARENT_TEST_REPORT_FILE="${GITHUB_WORKSPACE}/allure-results/$WORKFLOW_ID/test_report_"$group"_01.xml"
      for ALLURE_SUITE_FILE in $(find ${GITHUB_WORKSPACE}/allure-results/$WORKFLOW_ID/test_report_* -type f \
              -name 'test_report_'$group'_?[0-9].xml' \
            ! -name 'test_report_'$group'_01.xml'); do
          local TEST_CASES=$(sed '1,/<\/properties/d;/<\/testsuite/,$d' $ALLURE_SUITE_FILE)
          # insert extracted test cases into parent_test_report_file
          printf '%s\n' '0?<\/testcase>?a' $TEST_CASES . x | ex $PARENT_TEST_REPORT_FILE
      done
      #remove 'SynchronizedBeforeSuite' and 'AfterSuite' and other unnecessary xml tags from the final report
      cat <<<$(xmlstarlet ed -d '//testcase[@name="[SynchronizedBeforeSuite]" and @status="passed"]' $PARENT_TEST_REPORT_FILE) >$PARENT_TEST_REPORT_FILE
      cat <<<$(xmlstarlet ed -d '//testcase[@name="[AfterSuite]" and @status="passed"]' $PARENT_TEST_REPORT_FILE) >$PARENT_TEST_REPORT_FILE
      cat <<<$(xmlstarlet ed -d '//system-out' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
      sed -i 's/system-err/system-out/g' $PARENT_TEST_REPORT_FILE
      sed -i '/^.*END STEP:.*$/d; /Exit \[It\]/d; /AfterEach/d; /Aftereach/d' $PARENT_TEST_REPORT_FILE

      # for each test name verify status
      for TEST_NAME in $(xmlstarlet sel -t -v "//testcase/@name" $PARENT_TEST_REPORT_FILE); do
          local IS_PASSED=$(xmlstarlet sel -t -v 'count(//testcase[@name="'"${TEST_NAME}"'" and @status="passed"])' $PARENT_TEST_REPORT_FILE)
          local IS_FAILED=$(xmlstarlet sel -t -v 'count(//testcase[@name="'"${TEST_NAME}"'" and @status="failed"])' $PARENT_TEST_REPORT_FILE)
          if [[ "$IS_PASSED" -ge 1 || "$IS_FAILED" -ge 1 ]]; then
              # if test is 'passed' or 'failed' then remove all tests with 'skipped' status and remove duplicated tags with 'passed' and 'failed' statuses except one
              cat <<<$(xmlstarlet ed -d '//testcase[@name="'"${TEST_NAME}"'" and @status="skipped"]' $PARENT_TEST_REPORT_FILE) >$PARENT_TEST_REPORT_FILE
              cat <<<$(xmlstarlet ed -d '(//testcase[@name="'"${TEST_NAME}"'" and @status="passed"])[position()>1]' $PARENT_TEST_REPORT_FILE) >$PARENT_TEST_REPORT_FILE
              cat <<<$(xmlstarlet ed -d '(//testcase[@name="'"${TEST_NAME}"'" and @status="failed"])[position()>1]' $PARENT_TEST_REPORT_FILE) >$PARENT_TEST_REPORT_FILE
          else
              # if tests in not in 'passed' or 'failed' statuses, then remove all duplicated tags with 'skipped' statuses except one
              cat <<<$(xmlstarlet ed -d '(//testcase[@name="'"${TEST_NAME}"'" and @status="skipped"])[position()>1]' $PARENT_TEST_REPORT_FILE) >$PARENT_TEST_REPORT_FILE
          fi
      done
      # count 'total' and 'skipped' number of tests and update the values in the final report
      local TOTAL_TESTS=$(xmlstarlet sel -t -v 'count(//testcase)' $PARENT_TEST_REPORT_FILE)
      local FAILED_TESTS=$(xmlstarlet sel -t -v 'count(//testcase[@status="failed"])' $PARENT_TEST_REPORT_FILE)
      local BROKEN_TESTS=$(xmlstarlet sel -t -v 'count(//testcase[@status="broken"])' $PARENT_TEST_REPORT_FILE)
      local SKIPPED_TESTS=$(xmlstarlet sel -t -v 'count(//testcase[@status="skipped"])' $PARENT_TEST_REPORT_FILE)
      sed -i 's/tests="[^"]*/tests="'$TOTAL_TESTS'/g' $PARENT_TEST_REPORT_FILE
      sed -i 's/failures="[^"]*/failures="'$FAILED_TESTS'/g' $PARENT_TEST_REPORT_FILE
      sed -i 's/errors="[^"]*/errors="'$BROKEN_TESTS'/g' $PARENT_TEST_REPORT_FILE
      sed -i 's/skipped="[^"]*/skipped="'$SKIPPED_TESTS'/g' $PARENT_TEST_REPORT_FILE
      # remove all test report files except parent one for further processing
      find ${GITHUB_WORKSPACE}/allure-results/$WORKFLOW_ID/test_report_* -type f -name 'test_report_'$group'_?[0-9].xml' ! -name 'test_report_'$group'_01.xml' -delete
  done
}

# Function clean up the final test JSON files: removes all 'steps' objects that don't contain the 'name' object, 'Text' word in the name object, and CR_ID text.
# It will also remove the 'By' annotation, converts 'Duration' time (h,m,s) into 'ms', added a URL with an error line which is the point to the source code, and finally added a direct link into the log system.
# Takes a single argument - the WORKFLOW_ID (kind,gke,eks, aks etc.)
update_test_files()
{
      local WORKFLOW_ID=$1
      local CLUSTER_NAME=$2
      local REPOSITORY_OWNER=$3
      local BEGIN_TIME=$(date +%s000 -d "- 3 hours")
      local END_TIME=$(date +%s000 -d "+ 1 hours")
      cd allure-history/$WORKFLOW_ID/${GITHUB_RUN_NUMBER}/data/test-cases
      local GRAFANA_BASE_URL="https://hazelcastoperator.grafana.net"
      for i in $(ls); do
          cat <<< $(jq -e 'del(.testStage.steps[] | select(has("name") and (.name | startswith("STEP:")|not) and (.name | contains("CR_ID") | not)))
                               |.testStage.steps[].name |= (sub("STEP: "; "") | sub(" - /home.*$"; "")
                               | if . == "" then . else ((.[0:1] | ascii_upcase) + .[1:]) end)
                               |.testStage.steps[]+={status: "passed"}
                               |(if .status=="failed" then .+={links: [.statusTrace|split("\n")
                               |to_entries
                               |walk(if type == "object" and (.value | select(contains("hazelcast-platform-operator/hazelcast-platform-operator"))) then . else . end)
                               |del(.[].key)
                               |.[].value|=sub(" ";"")
                               |.[].value|= sub("/home/runner/work/hazelcast-platform-operator/hazelcast-platform-operator";"https://github.com/'${REPOSITORY_OWNER}'/hazelcast-platform-operator/blob/main")
                               |.[].value|= sub(".go:";".go#L")
                               |.[].value|=sub("In\\[It\\] at: ";"")
                               |.[].value|= sub(" @.*$"; "")
                               |unique
                               |to_entries[]
                               |.+={name: ("ERROR_LINE"+ "_" + (.key|tonumber+1|tostring))}
                               |.url+=.value[]
                               |del(.key)|del(.value)
                               |.+={type: "issue"}]}
                               |.testStage.steps[-1]+={status: "failed"} else . end)' $i) > $i

         local NUMBER_OF_TEST_RUNS=$(jq -r '[(.testStage.steps |to_entries[]| select(.value.name | select(contains("Setting the label and CR with name"))))] | length' $i)
         if [[ ${NUMBER_OF_TEST_RUNS} -gt 1 ]]; then
               local START_INDEX_OF_LAST_RETRY=$(jq -r '[(.testStage.steps |to_entries[]| select(.value.name | select(contains("Setting the label and CR with name"))))][-1].key' $i)
               cat <<< $(jq -e 'del(.testStage.steps[0:'${START_INDEX_OF_LAST_RETRY}'])' $i) > $i
         fi
         local TEST_STATUS=$(jq -r '.status' $i)
         if [[ ${TEST_STATUS} != "skipped" ]]; then
            cat <<< $(jq -e '.extra.tags={"tag": .testStage.steps[].name | select(contains("CR_ID")) | sub("CR_ID:"; "")| sub(" .*$"; "")}|del(.testStage.steps[] | select(.name | select(contains("CR_ID"))))' $i) > $i
            local CR_ID=$(jq -r '.extra.tags.tag' $i)
            local LINK=$(echo $GRAFANA_BASE_URL\/d\/-Lz9w3p4z\/all-logs\?orgId=1\&var-cluster="$CLUSTER_NAME"\&var-cr_id="$CR_ID"\&var-text=\&from="$BEGIN_TIME"\&to="$END_TIME")
            cat <<< $(jq -e '.links|= [{"name":"LOGS","url":"'"$LINK"'",type: "tms"}] + .' $i) > $i
        fi
      done
}

#This function sync certification tags and add the latest tag to the published image
sync_certificated_image_tags()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    FILTER="filter=deleted==false;repositories.published==true;repositories.tags.name==${VERSION}"
    CERT_IMAGE_ID=$(curl -X 'GET' --silent \
        -H "X-API-KEY: $RHEL_API_KEY" \
        -H 'Content-Type: application/json' \
        "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&page_size=100&page=0" | jq -r '.data[] | select(.container_grades.status == "completed") | ._id')

     curl -X 'POST' \
     "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/requests/images" \
     -H 'accept: application/json' \
     -H 'Content-Type: application/json' \
     -d "{
           \"image_id\": \"${CERT_IMAGE_ID}\",
           \"operation\": \"sync-tags\"
         }" \
     -H "X-API-KEY: ${RHEL_API_KEY}"
}

# It cleans up resources (all, PVCs and their bounded PVs) in the given namespace.
cleanup_namespace(){
  # number of all resources excepting `kubernetes` svc
  number_of_all_resources() {
    kubectl get all --namespace="$1" -o json | \
      jq '.items | map(select((.kind | contains("Service")) and (.metadata.name | contains("kubernetes")) | not)) | length'
  }

  # number of PVCs
  number_of_pvc() {
    kubectl get pvc --namespace="$1" -o json | \
      jq '.items | length'
  }

  # space separated list of PVs which are bounded to PVCs in given namespace
  list_of_bounded_pv(){
    kubectl get pvc --namespace="$1" -o json | \
      jq -r '.items[].spec.volumeName' | \
      tr '\n' ' '
  }

  ns="$1"

  if [ -z "$ns" ];
  then
      echo "namespace is not passed"
      exit 1
  fi

  echo "namespace: '$ns'"

  while [ "$(number_of_all_resources "$ns")" -gt 0 ]
  do
    echo "kubectl delete all"
    kubectl delete all --all --namespace="$ns"
    sleep 3
  done

  if [ "$(number_of_pvc "$ns")" -gt 0 ];
  then
    pvList="$(list_of_bounded_pv "$ns")"
    echo "kubectl delete PVCs"
    kubectl delete pvc --all --namespace="$ns"
    echo "kubectl delete PV $pvList"
    kubectl delete pv $pvList || true
  fi
}

wait_condition_pod_ready() {
  namespace="$1"
  pod_label="$2"
  try="$3"
  duration="$4"

  echo "namespace: $namespace, pod label: $pod_label"

  for ((i=1; i<=$try; i++)); do
    exit_status=0
    kubectl wait --for=condition=ready pod -n $namespace -l $pod_label --timeout 3s || exit_status=$?

    # If exit status is 0, break out of the loop
    if [ $exit_status -eq 0 ]; then
        echo "The pods labeled with '$pod_label' in '$namespace' namespace are ready"
        return 0
    fi

    sleep "$duration"
  done

  # If the loop completes without finding a successful exit status, return a non-zero exit status
  echo "The pods labeled with '$pod_label' in '$namespace' namespace are not ready yet"
  return 1
}

get_operator_pod_name(){
    namespace=$1
    podName=$(kubectl get pod --selector='app.kubernetes.io/name=hazelcast-platform-operator' --namespace=$namespace -o=jsonpath='{.items[*].metadata.name}')
    if [ -z "$podName" ]; then
        echo "operator pod not found in '$namespace' namespace"
        return 1
    fi
    echo $podName | grep -q " "
    if [ $? -eq 0 ]; then
        echo "there are multiple operator pods in the namespace '$namespace'"
        return 1
    fi
    echo $podName
}

get_pod_restart_count(){
    namespace=$1
    podName=$2
    kubectl get pod $podName --namespace=$namespace -o=jsonpath='{.status.containerStatuses[0].restartCount}'
}

assert_operator_pod_not_restarted(){
    namespace=$1
    operatorPod=$(get_operator_pod_name $namespace)
    echo "operator pod name: $operatorPod"
    if [ $? -ne 0 ]; then
        echo $operatorPod
        return 1
    fi
    restartCount=$(get_pod_restart_count $namespace $operatorPod)
    if [ $? -ne 0 ]; then
        echo $restartCount
        return 1
    fi
    echo "restart count: $restartCount"
    if [[ $restartCount -ne 0 ]]; then
        echo "restart count is not zero"
        return 1
    fi
}

get_hetzner_ip_ranges() {
  echo "108.62.0.0/16,135.181.0.0/16,136.243.0.0/16,144.76.0.0/16,148.251.0.0/16,172.241.0.0/16,178.63.0.0/16,188.40.0.0/16,5.9.0.0/16,95.216.0.0/16,95.217.0.0/16"
}

update_eks_security_group() {
  local AWS_REGION=$1
  local CLUSTER_NAME=$2

  local IP_RANGES=$(get_hetzner_ip_ranges | tr ',' '\n')
  local JSON_OUTPUT='{
    "IpPermissions": [
      {
        "IpProtocol": "tcp",
        "FromPort": 30000,
        "ToPort": 32767,
        "IpRanges": ['

  while read -r CIDR; do
    JSON_OUTPUT+='{"CidrIp": "'$CIDR'"},'
  done <<< "$IP_RANGES"

  # Remove the last comma and close the JSON structure
  JSON_OUTPUT=${JSON_OUTPUT%,}
  JSON_OUTPUT+=']
      }
    ]
  }'

  echo "$JSON_OUTPUT" > /tmp/ip-ranges.json
  local SECURITY_GROUP_ID=$(aws eks describe-cluster --region="$AWS_REGION" --name="$CLUSTER_NAME" --query="cluster.resourcesVpcConfig.clusterSecurityGroupId" --output=text --no-cli-pager)
  aws ec2 authorize-security-group-ingress \
    --region "$AWS_REGION" \
    --group-id "$SECURITY_GROUP_ID" \
    --cli-input-json file:///tmp/ip-ranges.json --no-cli-pager

  echo "Ingress rules applied successfully to security group: $SECURITY_GROUP_ID"
}

update_gke_firewall_rule() {
  local GCP_PROJECT_ID=$1
  local FIREWALL_RULE_NAME=$2

  IP_RANGES=$(get_hetzner_ip_ranges)
  gcloud compute firewall-rules update $FIREWALL_RULE_NAME \
    --source-ranges=${IP_RANGES} \
    --project $GCP_PROJECT_ID

  echo "Firewall rule updated successfully with IP ranges for rule: $FIREWALL_RULE_NAME"
}

update_aks_nsg_rule() {
  local NRG_NAME=$1
  local NSG_NAME=$2
  local NSG_RULE_NAME=$3
  local IP_RANGES=$(get_hetzner_ip_ranges | tr ',' ' ' | sed 's/ /","/g' | sed 's/^/"/;s/$/"/')

  az network nsg rule create \
    --name=$NSG_RULE_NAME \
    --nsg-name=${NSG_NAME} \
    --source-address-prefixes="[$IP_RANGES]" \
    --priority=101 \
    --resource-group=${NRG_NAME} \
    --access=Allow \
    --destination-port-ranges=30000-32767 \
    --direction=Inbound \
    --description="Allow Ubicloud connection" \
    --protocol=Tcp
  echo "NSG rule '$NSG_RULE_NAME' updated successfully in NSG '$NSG_NAME' with source IP ranges."
}

get_docker_latest_tag() {
  LATEST_TAG=$(curl -s "https://hub.docker.com/v2/repositories/$1/tags/?page_size=100" \
  | jq -r '[.results[] | select(.name | test("\\d+\\.\\d+\\.\\d+$")) | {name, last_updated}] | sort_by(.name) | last.name')
  echo $LATEST_TAG
}

get_docker_latest_snapshot_tag() {
  LATEST_SNAPSHOT_TAG=$(curl -s "https://hub.docker.com/v2/repositories/$1/tags/?page_size=100" \
  | jq -r '[.results[] | select(.name | test("\\d+\\.\\d+\\.\\d+-SNAPSHOT$")) | {name, last_updated}] | sort_by(.name) | last.name')
  echo $LATEST_SNAPSHOT_TAG
}

# accept docker repo name
update_version() {
  sed -i '/Version of Hazelcast Platform\./{n;s/\+kubebuilder:default:= *"[^"]*"/+kubebuilder:default:="'"$1"'"/;}' ${GITHUB_WORKSPACE}/api/v1alpha1/hazelcast_types.go
  sed -i '/Version of Management Center\./{n;s/\+kubebuilder:default:= *"[^"]*"/+kubebuilder:default:="'"$2"'"/;}' ${GITHUB_WORKSPACE}/api/v1alpha1/managementcenter_types.go
  sed -i 's/HazelcastVersion = \".*\"/HazelcastVersion = \"'$1'\"/' ${GITHUB_WORKSPACE}/internal/naming/constants.go
  sed -i 's/MCVersion = \".*\"/MCVersion = \"'$2'\"/' ${GITHUB_WORKSPACE}/internal/naming/constants.go
}


