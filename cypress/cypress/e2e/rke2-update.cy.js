/// <reference types="cypress" />

describe('Rancher updating functionality', () => {
  beforeEach(function () {
    cy.login()
  })

  it('upgrades all nodes', () => {
    cy.visit("/")
    cy.waitTableLoaded()
    cy.contains('test-cluster').click()
    cy.contains('Cluster Dashboard').should('be.visible')
    cy.waitTableLoaded()
    cy.contains('Nodes').click()
    cy.waitTableLoaded()

    cy.exec(`cd ..; ./util/upgrade_downstream_rke.sh`, {timeout: 60 * 60_000}).then((result) => {
      cy.log(result.stdout)
    })
  })
})
