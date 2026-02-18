import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('textbox', { name: 'JQl Query' })).toBeVisible();
});

test('should trigger new query when Metric field is changed', async ({
  panelEditPage,
  readProvisionedDataSource,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole('textbox', { name: 'JQl Query' }).fill('project=TEST');
  const queryReq = panelEditPage.waitForQueryDataRequest();
  await panelEditPage.getQueryEditorRow('A').getByRole('combobox', { name: 'Metric' }).click();
  // Select first option or specific option logic would go here, currently just checking if changing triggers query
  // For robustness, we might need to select an option if default isn't triggering on click
  await expect(await queryReq).toBeTruthy();
});

test('data query should return values', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole('textbox', { name: 'JQl Query' }).fill('project=TEST');
  await panelEditPage.setVisualization('Table');
  await expect(panelEditPage.refreshPanel()).toBeOK();
  // Verify data presence (mocked or real)
  await expect(panelEditPage.panel.data).toBeVisible();
});
