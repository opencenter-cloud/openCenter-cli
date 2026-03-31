apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: keycloak-subscription
  namespace: keycloak
spec:
  startingCSV: keycloak-operator.v26.4.2
