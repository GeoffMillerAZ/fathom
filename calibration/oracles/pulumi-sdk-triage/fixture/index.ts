import { Bucket as AwsBucket } from "@pulumi/aws/s3/bucket";
import { ResourceGroup } from "@pulumi/azure-native/resources/resourceGroup";
import { Bucket as GcpBucket } from "@pulumi/gcp/storage/bucket";

export function cloudResources() {
  const aws = new AwsBucket("probe", {});
  const azure = new ResourceGroup("probe", { location: "westus2" });
  const gcp = new GcpBucket("probe", { location: "US" });
  return { aws, azure, gcp };
}
